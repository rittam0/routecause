package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"whyfi/internal/platform"
)

type ServerOptions struct {
	Manager       *Manager
	Assets        fs.FS
	AllowedHosts  []string
	DemoMode      bool
	Quick         time.Duration
	Full          time.Duration
	Capabilities  func() platform.CapabilityReport
	MaxSSEClients int32
}

type Server struct {
	options      ServerOptions
	sseClients   atomic.Int32
	capabilities platform.CapabilityReport
}

func NewServer(options ServerOptions) *Server {
	if options.MaxSSEClients == 0 {
		options.MaxSSEClients = 8
	}
	if options.Capabilities == nil {
		options.Capabilities = func() platform.CapabilityReport { return platform.Capabilities(context.Background(), time.Second) }
	}
	return &Server{options: options, capabilities: options.Capabilities()}
}

func (s *Server) Handler() http.Handler { return s.security(http.HandlerFunc(s.route)) }

func (s *Server) security(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, host := range s.options.AllowedHosts {
		allowed[strings.ToLower(host)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[strings.ToLower(r.Host)] {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "http" || !allowed[strings.ToLower(u.Host)] {
				http.Error(w, "invalid origin", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		s.static(w, r)
		return
	}
	switch {
	case r.URL.Path == "/api/v1/capabilities" && r.Method == http.MethodGet:
		s.capabilityResponse(w)
	case r.URL.Path == "/api/v1/diagnoses" && r.Method == http.MethodPost:
		s.start(w, r)
	case r.URL.Path == "/api/v1/history" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.options.Manager.History())
	case strings.HasPrefix(r.URL.Path, "/api/v1/diagnoses/"):
		s.diagnosisRoute(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (s *Server) capabilityResponse(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{"platform": s.capabilities, "demo_mode": s.options.DemoMode, "quick_seconds": s.options.Quick.Seconds(), "full_seconds": s.options.Full.Seconds(), "demo_scenarios": []string{"healthy", "dns", "wifi", "gateway", "upstream", "destination", "unknown"}})
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request StartRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	record, err := s.options.Manager.Start(request)
	if errors.Is(err, ErrAlreadyRunning) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "diagnosis already running"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, StartResponse{ID: record.id, EventsURL: "/api/v1/diagnoses/" + record.id + "/events", SessionURL: "/api/v1/diagnoses/" + record.id})
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON value")
	}
	return nil
}

func (s *Server) diagnosisRoute(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/v1/diagnoses/")
	parts := strings.Split(remainder, "/")
	if len(parts) < 1 || parts[0] == "" || strings.Contains(parts[0], "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	record := s.options.Manager.get(parts[0])
	if record == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if len(parts) == 2 && parts[1] == "events" && r.Method == http.MethodGet {
		s.events(w, r, record)
		return
	}
	if len(parts) == 2 && parts[1] == "export" && r.Method == http.MethodGet {
		s.export(w, record)
		return
	}
	if len(parts) != 1 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.options.Manager.Cancel(record.id); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	record.mu.Lock()
	status, session, item := record.status, record.session, record.historyItemLocked()
	record.mu.Unlock()
	if session == nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"status": status, "session": item})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (r *record) historyItemLocked() HistoryItem {
	item := HistoryItem{ID: r.id, StartedAt: r.started, EndedAt: r.ended, Duration: r.duration.String(), Status: r.status, Demo: r.demo, Scenario: r.scenario, Layers: cloneLayers(r.layers)}
	if r.session != nil {
		item.Code, item.Conclusion, item.Confidence = r.session.Diagnosis.Code, r.session.Diagnosis.Conclusion, r.session.Diagnosis.Confidence
	}
	return item
}

func (s *Server) export(w http.ResponseWriter, record *record) {
	record.mu.Lock()
	session := record.session
	record.mu.Unlock()
	if session == nil {
		http.Error(w, "session is not complete", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=routecause-%s.json", record.id))
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request, record *record) {
	if s.sseClients.Add(1) > s.options.MaxSSEClients {
		s.sseClients.Add(-1)
		http.Error(w, "too many event clients", http.StatusTooManyRequests)
		return
	}
	defer s.sseClients.Add(-1)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	after := 0
	if value := r.Header.Get("Last-Event-ID"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			http.Error(w, "invalid Last-Event-ID", http.StatusBadRequest)
			return
		}
		after = parsed
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	for {
		events, notify, status := record.snapshot(after)
		for _, event := range events {
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data)
			after = event.ID
		}
		flusher.Flush()
		if status == "completed" || status == "cancelled" || status == "failed" {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-notify:
		case <-time.After(15 * time.Second):
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(s.options.Assets, name)
	if err != nil {
		data, err = fs.ReadFile(s.options.Assets, "index.html")
	}
	if err != nil {
		http.Error(w, "interface unavailable", http.StatusInternalServerError)
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func IsLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	return err == nil && net.ParseIP(host).IsLoopback()
}
