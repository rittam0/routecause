package session

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"whyfi/internal/collector"
	"whyfi/internal/config"
	"whyfi/internal/model"
)

type fakeProbe struct {
	kind, target string
	run          func(context.Context) model.ProbeResult
}

func (p fakeProbe) Type() string                                  { return p.kind }
func (p fakeProbe) Target() string                                { return p.target }
func (p fakeProbe) Collect(ctx context.Context) model.ProbeResult { return p.run(ctx) }

func TestCancellationDuringStaggeredSchedules(t *testing.T) {
	var stopped atomic.Int32
	probes := []collector.Probe{}
	for i := 0; i < 3; i++ {
		probes = append(probes, fakeProbe{kind: "fake", target: string(rune('a' + i)), run: func(ctx context.Context) model.ProbeResult {
			<-ctx.Done()
			stopped.Add(1)
			return model.ProbeResult{CollectorType: "fake", ErrorCategory: model.ErrorCancelled}
		}})
	}
	cfg := testConfig()
	cfg.MaxConcurrency = 3
	cfg.Duration = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_, err := NewRunnerWithProbes(cfg, collector.RealClock{}, quietLogger(), probes).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Load() != 1 {
		t.Fatalf("expected only the first staggered collector to start, got %d", stopped.Load())
	}
}

func TestTimedOutCollectorDoesNotBlockAnother(t *testing.T) {
	probes := []collector.Probe{fakeProbe{kind: "slow", target: "slow", run: func(ctx context.Context) model.ProbeResult {
		<-ctx.Done()
		return model.ProbeResult{CollectorType: "slow", Target: "slow", ErrorCategory: model.ErrorTimeout}
	}}, fakeProbe{kind: "fast", target: "fast", run: func(context.Context) model.ProbeResult {
		return model.ProbeResult{CollectorType: "fast", Target: "fast", Success: true}
	}}}
	cfg := testConfig()
	cfg.Duration = 300 * time.Millisecond
	cfg.ProbeTimeout = 20 * time.Millisecond
	cfg.MaxConcurrency = 2
	got, err := NewRunnerWithProbes(cfg, collector.RealClock{}, quietLogger(), probes).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, result := range got.RawResults {
		if result.CollectorType == "fast" && result.Success {
			found = true
		}
	}
	if !found {
		t.Fatal("fast collector did not complete")
	}
}

func TestShortSessionReportsInsufficientEvidence(t *testing.T) {
	probe := fakeProbe{kind: "https", target: "health", run: func(context.Context) model.ProbeResult {
		return model.ProbeResult{CollectorType: "https", Target: "health", Success: true, Values: map[string]float64{"total_ms": 10}}
	}}
	cfg := testConfig()
	cfg.Duration = 40 * time.Millisecond
	got, err := NewRunnerWithProbes(cfg, collector.RealClock{}, quietLogger(), []collector.Probe{probe}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Diagnosis.Code != model.DiagnosisUnknown {
		t.Fatalf("got %s, want unknown", got.Diagnosis.Code)
	}
}

func testConfig() config.Config {
	c := config.Default()
	c.DeviceInterval, c.DNSInterval, c.TCPInterval, c.HTTPSInterval, c.WiFiInterval, c.GatewayInterval = time.Hour, time.Hour, time.Hour, time.Hour, time.Hour, time.Hour
	return c
}
func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
