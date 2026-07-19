package session

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"whyfi/internal/collector"
	"whyfi/internal/config"
	"whyfi/internal/diagnosis"
	"whyfi/internal/model"
	"whyfi/internal/platform"
)

type Runner struct {
	cfg              config.Config
	clock            collector.Clock
	logger           *slog.Logger
	configuredProbes []collector.Probe
	observer         func(model.ProbeResult)
}

func (r *Runner) WithObserver(observer func(model.ProbeResult)) *Runner {
	r.observer = observer
	return r
}

func NewRunner(cfg config.Config, clock collector.Clock, logger *slog.Logger) *Runner {
	return &Runner{cfg: cfg, clock: clock, logger: logger}
}

// NewRunnerWithProbes supports deterministic tests and platform-specific probe sets.
func NewRunnerWithProbes(cfg config.Config, clock collector.Clock, logger *slog.Logger, probes []collector.Probe) *Runner {
	return &Runner{cfg: cfg, clock: clock, logger: logger, configuredProbes: probes}
}

func (r *Runner) Run(parent context.Context) (model.DiagnosticSession, error) {
	if err := r.cfg.Validate(); err != nil {
		return model.DiagnosticSession{}, err
	}
	started := r.clock.Now()
	var sequence atomic.Uint64
	id := func() string { return fmt.Sprintf("%d-%06d", started.UnixMilli(), sequence.Add(1)) }
	probes := r.configuredProbes
	if probes == nil {
		probes = r.probes(id)
	}
	ctx, cancel := context.WithTimeout(parent, r.cfg.Duration)
	defer cancel()
	jobs := make(chan collector.Probe)
	results := make(chan model.ProbeResult, len(probes)*2)
	var workers sync.WaitGroup
	for range r.cfg.MaxConcurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for probe := range jobs {
				probeCtx, probeCancel := context.WithTimeout(ctx, r.cfg.ProbeTimeout)
				result := probe.Collect(probeCtx)
				probeCancel()
				if r.observer != nil {
					r.observer(result)
				}
				select {
				case results <- result:
				case <-ctx.Done():
					results <- result
				}
				r.logger.Debug("probe completed", "collector", result.CollectorType, "target", result.Target, "success", result.Success, "error_category", result.ErrorCategory)
			}
		}()
	}
	var schedulers sync.WaitGroup
	for i, probe := range probes {
		schedulers.Add(1)
		go func(index int, probe collector.Probe) {
			defer schedulers.Done()
			interval := r.intervalFor(probe.Type())
			// Spread first attempts over at most one second. This avoids synchronized
			// bursts while ensuring short development sessions still exercise all probes.
			offset := time.Duration(index) * 125 * time.Millisecond
			if offset > time.Second {
				offset = time.Second
			}
			if !wait(ctx, offset) {
				return
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case jobs <- probe:
				case <-ctx.Done():
					return
				}
				if oneShot, ok := probe.(collector.OneShotProbe); ok && oneShot.OneShot() {
					return
				}
				select {
				case <-ticker.C:
				case <-ctx.Done():
					return
				}
			}
		}(i, probe)
	}
	scheduled := make(chan struct{})
	go func() { schedulers.Wait(); close(jobs); close(scheduled) }()
	done := make(chan struct{})
	go func() { <-scheduled; workers.Wait(); close(results); close(done) }()
	all := []model.ProbeResult{}
	for result := range results {
		all = append(all, result)
	}
	<-done
	summaries := Summarize(all)
	conclusion, evidence := diagnosis.New(r.cfg.Thresholds).Diagnose(summaries)
	return model.DiagnosticSession{SchemaVersion: model.SchemaVersion, ID: id(), Configuration: r.cfg.Snapshot(), StartedAt: started, EndedAt: r.clock.Now(), RawResults: all, Summaries: summaries, Evidence: evidence, Diagnosis: conclusion, Limitations: limitations(platform.NewNetworkAPIs().NativeWindows)}, nil
}

func (r *Runner) intervalFor(kind string) time.Duration {
	switch kind {
	case "device_resources":
		return r.cfg.DeviceInterval
	case "dns_system", "dns_comparison":
		return r.cfg.DNSInterval
	case "tcp_connect":
		return r.cfg.TCPInterval
	case "https":
		return r.cfg.HTTPSInterval
	case "wifi":
		return r.cfg.WiFiInterval
	case "gateway":
		return r.cfg.GatewayInterval
	default:
		return r.cfg.DeviceInterval
	}
}

func wait(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (r *Runner) probes(id collector.IDGenerator) []collector.Probe {
	probes := []collector.Probe{collector.NewSystemDNS(r.cfg.DNSHostname, r.clock, id), r.deviceProbe(runtime.GOOS, id)}
	for _, resolver := range r.cfg.ComparisonResolvers {
		probes = append(probes, collector.NewComparisonDNS(r.cfg.DNSHostname, resolver, r.clock, id))
	}
	for _, target := range r.cfg.TCPDestinations {
		probes = append(probes, collector.NewTCP(target, r.clock, id))
	}
	probes = append(probes,
		collector.NewHTTPS(r.cfg.HTTPSHealthEndpoint, r.cfg.HTTPSHealthMethod, r.cfg.HTTPSHealthStatuses, r.clock, id),
		collector.NewHTTPS(r.cfg.HTTPSSecondaryEndpoint, r.cfg.HTTPSSecondaryMethod, r.cfg.HTTPSSecondaryStatuses, r.clock, id),
	)
	local := platform.NewNetworkAPIs()
	probes = append(probes, collector.NewWiFi(local.WiFi, r.clock, id), collector.NewGateway(local.Route, local.Echo, r.cfg.ProbeTimeout, r.clock, id))
	return probes
}

func (r *Runner) deviceProbe(goos string, id collector.IDGenerator) collector.Probe {
	if goos != "linux" {
		return collector.NewUnsupported("device_resources", "local_device", r.clock, id)
	}
	return collector.NewDevice(collector.ProcSystemReader{}, r.clock, id)
}

func limitations(nativeWindows bool) []string {
	items := []string{"Public destinations can rate-limit, redirect, or fail independently of the user's connection.", "A quiet measurement window cannot rule out intermittent degradation.", "RouteCause does not infer ISP fault from traceroute behavior and does not collect traceroute in milestone 1.5."}
	if !nativeWindows {
		items = append(items, "Physical Wi-Fi and Windows gateway evidence are unsupported on this platform; WSL measurements describe only the WSL network path.")
	} else {
		items = append(items, "Device resource pressure measurements are unsupported on native Windows; no CPU or memory values were inferred from Linux semantics.")
	}
	return items
}
