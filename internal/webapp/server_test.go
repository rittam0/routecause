package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"whyfi/internal/model"
	"whyfi/internal/platform"
)

type runnerFunc func(context.Context, time.Duration, func(model.ProbeResult)) (model.DiagnosticSession, error)

func (f runnerFunc) Run(ctx context.Context, duration time.Duration, observer func(model.ProbeResult)) (model.DiagnosticSession, error) {
	return f(ctx, duration, observer)
}

func completedRunner() Runner {
	return runnerFunc(func(_ context.Context, _ time.Duration, observer func(model.ProbeResult)) (model.DiagnosticSession, error) {
		observer(model.ProbeResult{CollectorType: "wifi", Target: "private-ssid", Success: true})
		session := fixtureSession(model.DiagnosisHealthy)
		session.RawResults = []model.ProbeResult{{CollectorType: "wifi", Target: "private-ssid", Success: true, Details: map[string]string{"network_label": "private-ssid", "bssid": "00:11:22:33:44:55", "guid": "secret-guid"}}}
		session.Summaries = []model.CollectorSummary{{CollectorType: "wifi", Target: "private-ssid", Attempts: 3, Successes: 3, SuccessRate: 1}}
		return session, nil
	})
}

func blockingRunner() Runner {
	return runnerFunc(func(ctx context.Context, _ time.Duration, observer func(model.ProbeResult)) (model.DiagnosticSession, error) {
		observer(model.ProbeResult{CollectorType: "device_resources", Success: true})
		<-ctx.Done()
		return model.DiagnosticSession{}, ctx.Err()
	})
}

func testServer(t *testing.T, runner Runner, demo bool) (*Server, *Manager) {
	t.Helper()
	manager := NewManager(runner, completedRunner(), demo, 15*time.Second, 60*time.Second)
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>RouteCause</html>")}}
	server := NewServer(ServerOptions{Manager: manager, Assets: fs.FS(assets), AllowedHosts: []string{"127.0.0.1:8765"}, DemoMode: demo, Quick: 15 * time.Second, Full: 60 * time.Second, Capabilities: func() platform.CapabilityReport { return platform.CapabilityReport{} }, MaxSSEClients: 2})
	return server, manager
}

func request(handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:8765"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func startSession(t *testing.T, handler http.Handler, body string) StartResponse {
	t.Helper()
	response := request(handler, http.MethodPost, "/api/v1/diagnoses", body)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start status %d: %s", response.Code, response.Body.String())
	}
	var started StartResponse
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	return started
}

func waitForStatus(t *testing.T, manager *Manager, id, status string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if record := manager.get(id); record != nil {
			record.mu.Lock()
			got := record.status
			record.mu.Unlock()
			if got == status {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("session %s did not reach %s", id, status)
}

func TestAPIStartCompletionRetrievalExportHistoryAndPrivacy(t *testing.T) {
	server, manager := testServer(t, completedRunner(), false)
	started := startSession(t, server.Handler(), `{"duration":"quick"}`)
	waitForStatus(t, manager, started.ID, "completed")
	result := request(server.Handler(), http.MethodGet, started.SessionURL, "")
	if result.Code != http.StatusOK {
		t.Fatalf("result status %d", result.Code)
	}
	text := strings.ToLower(result.Body.String())
	for _, forbidden := range []string{"private-ssid", "00:11:22:33:44:55", "secret-guid", "bssid"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("private value %q exposed: %s", forbidden, text)
		}
	}
	if !strings.Contains(result.Body.String(), "Wi-Fi network 1") {
		t.Fatal("redacted network label missing")
	}
	exported := request(server.Handler(), http.MethodGet, "/api/v1/diagnoses/"+started.ID+"/export", "")
	if exported.Code != http.StatusOK || !strings.Contains(exported.Header().Get("Content-Disposition"), ".json") {
		t.Fatalf("bad export: %d %#v", exported.Code, exported.Header())
	}
	history := request(server.Handler(), http.MethodGet, "/api/v1/history", "")
	if history.Code != http.StatusOK || strings.Contains(history.Body.String(), "raw_results") {
		t.Fatalf("unsafe history: %s", history.Body.String())
	}
}

func TestConcurrentStartRejectedAndCancellationCompletes(t *testing.T) {
	server, manager := testServer(t, blockingRunner(), false)
	started := startSession(t, server.Handler(), `{"duration":"full"}`)
	duplicate := request(server.Handler(), http.MethodPost, "/api/v1/diagnoses", `{"duration":"quick"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", duplicate.Code)
	}
	cancelled := request(server.Handler(), http.MethodDelete, started.SessionURL, "")
	if cancelled.Code != http.StatusNoContent {
		t.Fatalf("cancel status = %d", cancelled.Code)
	}
	waitForStatus(t, manager, started.ID, "cancelled")
}

func TestManagerShutdownCancelsActiveDiagnosis(t *testing.T) {
	_, manager := testServer(t, blockingRunner(), false)
	record, err := manager.Start(StartRequest{Duration: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	manager.Shutdown()
	waitForStatus(t, manager, record.id, "cancelled")
}

func TestSSEReplayAndReconnectFromLastEventID(t *testing.T) {
	server, manager := testServer(t, completedRunner(), false)
	started := startSession(t, server.Handler(), `{"duration":"quick"}`)
	waitForStatus(t, manager, started.ID, "completed")
	all := request(server.Handler(), http.MethodGet, started.EventsURL, "")
	if all.Code != http.StatusOK || !strings.Contains(all.Body.String(), "event: started") || !strings.Contains(all.Body.String(), "event: completed") {
		t.Fatalf("unexpected stream: %s", all.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, started.EventsURL, nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Last-Event-ID", "2")
	replay := httptest.NewRecorder()
	server.Handler().ServeHTTP(replay, req)
	if strings.Contains(replay.Body.String(), "event: started") || !strings.Contains(replay.Body.String(), "event: completed") {
		t.Fatalf("bad replay: %s", replay.Body.String())
	}
}

func TestSSEClientLimitAndInvalidReconnectID(t *testing.T) {
	server, manager := testServer(t, completedRunner(), false)
	started := startSession(t, server.Handler(), `{"duration":"quick"}`)
	waitForStatus(t, manager, started.ID, "completed")
	server.sseClients.Store(server.options.MaxSSEClients)
	limited := request(server.Handler(), http.MethodGet, started.EventsURL, "")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limit status %d", limited.Code)
	}
	server.sseClients.Store(0)
	req := httptest.NewRequest(http.MethodGet, started.EventsURL, nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Last-Event-ID", "not-a-number")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("reconnect status %d", recorder.Code)
	}
}

func TestMalformedInputAndSecurityRejections(t *testing.T) {
	server, _ := testServer(t, completedRunner(), false)
	handler := server.Handler()
	tests := []struct {
		name    string
		request *http.Request
		want    int
	}{
		{"bad host", httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil), http.StatusForbidden},
		{"unknown field", httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses", strings.NewReader(`{"duration":"quick","target":"evil"}`)), http.StatusBadRequest},
		{"bad duration", httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses", strings.NewReader(`{"duration":"forever"}`)), http.StatusBadRequest},
		{"extra JSON", httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses", strings.NewReader(`{"duration":"quick"}{}`)), http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.name != "bad host" {
				test.request.Host = "127.0.0.1:8765"
				test.request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, test.request)
			if recorder.Code != test.want {
				t.Fatalf("got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	origin := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	origin.Host = "127.0.0.1:8765"
	origin.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, origin)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("origin status %d", recorder.Code)
	}
	large := `{"duration":"quick","padding":"` + strings.Repeat("x", 5000) + `"}`
	response := request(handler, http.MethodPost, "/api/v1/diagnoses", large)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("large body status %d", response.Code)
	}
}

func TestDemoModeRequiresKnownScenarioAndLabelsHistory(t *testing.T) {
	server, manager := testServer(t, completedRunner(), true)
	invalid := request(server.Handler(), http.MethodPost, "/api/v1/diagnoses", `{"duration":"quick","demo_scenario":"real"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid demo status %d", invalid.Code)
	}
	started := startSession(t, server.Handler(), `{"duration":"quick","demo_scenario":"dns"}`)
	waitForStatus(t, manager, started.ID, "completed")
	history := manager.History()
	if len(history) != 1 || !history[0].Demo || history[0].Scenario != "dns" {
		t.Fatalf("demo was not labelled: %#v", history)
	}
}

func fixtureSession(code model.DiagnosisCode) model.DiagnosticSession {
	evidence := []model.Evidence{{ID: "fixture", Statement: "Fixture evidence.", Supports: true, SummaryKeys: []string{}}}
	return model.DiagnosticSession{SchemaVersion: model.SchemaVersion, ID: "fixture", StartedAt: time.Now(), EndedAt: time.Now(), Diagnosis: model.Diagnosis{Code: code, Conclusion: "Fixture conclusion.", Confidence: "moderate", SupportingEvidence: evidence, ContradictingEvidence: []model.Evidence{}, Alternatives: []string{"Fixture alternative."}}, Evidence: evidence, Limitations: []string{"Fixture limitation."}}
}

func TestDemoFixturesCoverEveryScenarioAndCancel(t *testing.T) {
	for scenario, want := range demoCodes {
		t.Run(scenario, func(t *testing.T) {
			session, err := scenarioRunner{scenario: scenario, runner: DemoRunner{}}.Run(context.Background(), time.Minute, func(model.ProbeResult) {})
			if err != nil || session.Diagnosis.Code != want {
				t.Fatalf("got %s, %v", session.Diagnosis.Code, err)
			}
			if !strings.Contains(session.Limitations[0], "DEMO MODE") {
				t.Fatal("demo fixture was not visibly labelled")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (scenarioRunner{scenario: "healthy", runner: DemoRunner{}}).Run(ctx, time.Minute, func(model.ProbeResult) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}

func TestExportUsesRouteCauseFilename(t *testing.T) {
	server, manager := testServer(t, completedRunner(), false)
	started := startSession(t, server.Handler(), `{"duration":"quick"}`)
	waitForStatus(t, manager, started.ID, "completed")
	exported := request(server.Handler(), http.MethodGet, "/api/v1/diagnoses/"+started.ID+"/export", "")
	value := exported.Header().Get("Content-Disposition")
	if !strings.Contains(value, "routecause-") || strings.Contains(value, "whyfi-") {
		t.Fatalf("Content-Disposition = %q", value)
	}
}
