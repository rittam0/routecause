package webapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"whyfi/internal/model"
)

var ErrAlreadyRunning = errors.New("a diagnosis is already running")

type record struct {
	mu       sync.Mutex
	id       string
	started  time.Time
	ended    *time.Time
	duration time.Duration
	status   string
	demo     bool
	scenario string
	layers   []LayerState
	events   []Event
	notify   chan struct{}
	cancel   context.CancelFunc
	session  *model.DiagnosticSession
	err      string
}

func (r *record) append(eventType string, data any) {
	r.mu.Lock()
	r.events = append(r.events, Event{ID: len(r.events) + 1, Type: eventType, Data: data})
	close(r.notify)
	r.notify = make(chan struct{})
	r.mu.Unlock()
}

func (r *record) snapshot(after int) ([]Event, <-chan struct{}, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	start := after
	if start < 0 {
		start = 0
	}
	if start > len(r.events) {
		start = len(r.events)
	}
	return append([]Event(nil), r.events[start:]...), r.notify, r.status
}

type Manager struct {
	mu         sync.Mutex
	runner     Runner
	demoRunner Runner
	demoMode   bool
	quick      time.Duration
	full       time.Duration
	active     *record
	records    map[string]*record
	history    []string
	sequence   atomic.Uint64
	clock      func() time.Time
}

func NewManager(runner, demoRunner Runner, demoMode bool, quick, full time.Duration) *Manager {
	return &Manager{runner: runner, demoRunner: demoRunner, demoMode: demoMode, quick: quick, full: full, records: map[string]*record{}, clock: time.Now}
}

func (m *Manager) Start(request StartRequest) (*record, error) {
	duration := m.quick
	if request.Duration == "full" {
		duration = m.full
	} else if request.Duration != "quick" {
		return nil, errors.New("duration must be quick or full")
	}
	if m.demoMode {
		if !validDemoScenario(request.DemoScenario) {
			return nil, errors.New("a valid demo_scenario is required in demo mode")
		}
	} else if request.DemoScenario != "" {
		return nil, errors.New("demo_scenario is unavailable outside demo mode")
	}
	m.mu.Lock()
	if m.active != nil {
		m.mu.Unlock()
		return nil, ErrAlreadyRunning
	}
	now := m.clock()
	id := fmt.Sprintf("web-%d-%04d", now.UnixMilli(), m.sequence.Add(1))
	ctx, cancel := context.WithCancel(context.Background())
	r := &record{id: id, started: now, duration: duration, status: "running", demo: m.demoMode, scenario: request.DemoScenario, layers: cloneLayers(layerOrder), notify: make(chan struct{}), cancel: cancel}
	m.active, m.records[id] = r, r
	m.history = append([]string{id}, m.history...)
	if len(m.history) > 10 {
		delete(m.records, m.history[len(m.history)-1])
		m.history = m.history[:10]
	}
	m.mu.Unlock()
	r.append("started", map[string]any{"id": id, "started_at": now, "duration_seconds": duration.Seconds(), "demo": r.demo, "demo_scenario": r.scenario, "layers": cloneLayers(r.layers)})
	selected := m.runner
	if m.demoMode {
		selected = scenarioRunner{scenario: request.DemoScenario, runner: m.demoRunner}
	}
	go m.execute(ctx, r, selected)
	return r, nil
}

func (m *Manager) execute(ctx context.Context, r *record, runner Runner) {
	seen := map[string]bool{}
	var seenMu sync.Mutex
	observe := func(result model.ProbeResult) {
		layer := layerForCollector(result.CollectorType)
		seenMu.Lock()
		if layer == "" || seen[layer] {
			seenMu.Unlock()
			return
		}
		seen[layer] = true
		seenMu.Unlock()
		r.setLayer(layer, "testing")
		r.append("layer", map[string]any{"layer": layer, "status": "testing"})
	}
	session, err := runner.Run(ctx, r.duration, observe)
	now := m.clock()
	r.mu.Lock()
	r.ended = &now
	if ctx.Err() != nil {
		r.status = "cancelled"
	} else if err != nil {
		r.status, r.err = "failed", "diagnosis failed"
	} else {
		safe := sanitizeSession(session)
		r.session, r.status = &safe, "completed"
		r.layers = finalLayers(safe)
	}
	status, layers, safeSession := r.status, cloneLayers(r.layers), r.session
	r.mu.Unlock()
	if status == "completed" {
		r.append("completed", map[string]any{"layers": layers, "diagnosis": safeSession.Diagnosis})
	} else {
		r.append(status, map[string]any{"layers": layers, "message": r.err})
	}
	m.mu.Lock()
	if m.active == r {
		m.active = nil
	}
	m.mu.Unlock()
}

func (r *record) setLayer(id, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.layers {
		if r.layers[i].ID == id {
			r.layers[i].Status = status
		}
	}
}

func (m *Manager) Cancel(id string) error {
	r := m.get(id)
	if r == nil {
		return errors.New("session not found")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status != "running" {
		return errors.New("session is not active")
	}
	r.status = "cancelling"
	r.cancel()
	return nil
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	active := m.active
	m.mu.Unlock()
	if active != nil {
		active.cancel()
	}
}

func (m *Manager) get(id string) *record { m.mu.Lock(); defer m.mu.Unlock(); return m.records[id] }

func (m *Manager) History() []HistoryItem {
	m.mu.Lock()
	ids := append([]string(nil), m.history...)
	m.mu.Unlock()
	out := make([]HistoryItem, 0, len(ids))
	for _, id := range ids {
		if r := m.get(id); r != nil {
			out = append(out, r.historyItem())
		}
	}
	return out
}

func (r *record) historyItem() HistoryItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := HistoryItem{ID: r.id, StartedAt: r.started, EndedAt: r.ended, Duration: r.duration.String(), Status: r.status, Demo: r.demo, Scenario: r.scenario, Layers: cloneLayers(r.layers)}
	if r.session != nil {
		item.Code, item.Conclusion, item.Confidence = r.session.Diagnosis.Code, r.session.Diagnosis.Conclusion, r.session.Diagnosis.Confidence
	}
	return item
}

func cloneLayers(values []LayerState) []LayerState { return append([]LayerState(nil), values...) }

func layerForCollector(kind string) string {
	switch kind {
	case "device_resources":
		return ""
	case "wifi":
		return "local"
	case "gateway":
		return "gateway"
	case "dns_system", "dns_comparison":
		return "dns"
	case "tcp_connect":
		return "internet"
	case "https":
		return "destinations"
	}
	return ""
}
