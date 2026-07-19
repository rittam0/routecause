package diagnosis

import (
	"testing"

	"whyfi/internal/config"
	"whyfi/internal/model"
)

func TestDiagnoses(t *testing.T) {
	tests := []struct {
		name      string
		summaries []model.CollectorSummary
		want      model.DiagnosisCode
	}{
		{"healthy", healthySummaries(), model.DiagnosisHealthy},
		{"slow system DNS with healthy comparison", replace(healthySummaries(), summary("dns_system", "example.com", "dns_ms", 220)), model.DiagnosisDNS},
		{"one slow destination", replace(healthySummaries(), summary("tcp_connect", "slow:443", "connect_ms", 400)), model.DiagnosisDestination},
		{"device pressure", replace(healthySummaries(), device(1.5, 30)), model.DiagnosisDevice},
		{"insufficient", []model.CollectorSummary{oneSample("dns_system", "example.com", "dns_ms", 20)}, model.DiagnosisUnknown},
	}
	engine := New(config.Default().Thresholds)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := engine.Diagnose(test.summaries)
			if got.Code != test.want {
				t.Fatalf("got %s, want %s: %#v", got.Code, test.want, got)
			}
			if test.want == model.DiagnosisUnknown && got.Confidence != "" {
				t.Fatalf("unknown diagnosis must withhold confidence, got %q", got.Confidence)
			}
		})
	}
}

func TestConflictingSignalsAreUnknown(t *testing.T) {
	s := healthySummaries()
	s = replace(s, summary("dns_system", "example.com", "dns_ms", 220))
	s = replace(s, device(1.5, 30))
	got, _ := New(config.Default().Thresholds).Diagnose(s)
	if got.Code != model.DiagnosisUnknown {
		t.Fatalf("got %s", got.Code)
	}
}

func TestHealthyTypicalWithIsolatedOutlier(t *testing.T) {
	s := healthySummaries()
	dns := s[0]
	stat := dns.Metrics["dns_ms"]
	stat.Max, stat.Mean, stat.MAD, stat.OutlierCount = 3000, 120, 2, 1
	dns.Metrics["dns_ms"] = stat
	s = replace(s, dns)
	got, evidence := New(config.Default().Thresholds).Diagnose(s)
	if got.Code != model.DiagnosisHealthy {
		t.Fatalf("got %s", got.Code)
	}
	if !hasEvidence(evidence, "isolated-instability") {
		t.Fatalf("missing isolated instability evidence: %#v", evidence)
	}
}

func TestRepeatedTimeoutsAreInconclusive(t *testing.T) {
	s := healthySummaries()
	dns := s[0]
	dns.Attempts, dns.Successes, dns.SuccessRate, dns.FailureRate, dns.TimeoutRate, dns.LongestFailureRun = 10, 7, .7, .3, .3, 2
	dns.Errors = map[model.ErrorCategory]int{model.ErrorTimeout: 3}
	s = replace(s, dns)
	got, evidence := New(config.Default().Thresholds).Diagnose(s)
	if got.Code != model.DiagnosisUnknown || !hasEvidence(evidence, "repeated-probe-failures") {
		t.Fatalf("unexpected diagnosis: %#v %#v", got, evidence)
	}
}

func TestSevereP95IsInconclusive(t *testing.T) {
	s := healthySummaries()
	tcp := s[2]
	stat := tcp.Metrics["connect_ms"]
	p95 := 500.0
	stat.P95 = &p95
	tcp.Metrics["connect_ms"] = stat
	s = replace(s, tcp)
	got, evidence := New(config.Default().Thresholds).Diagnose(s)
	if got.Code != model.DiagnosisUnknown || !hasEvidence(evidence, "severe-tail-latency") {
		t.Fatalf("unexpected diagnosis: %#v %#v", got, evidence)
	}
}

func TestLowerFrequencyHTTPSStillRequiresMinimumSamples(t *testing.T) {
	s := healthySummaries()
	https := s[4]
	https.Attempts, https.Successes = 1, 1
	stat := https.Metrics["total_ms"]
	stat.Count = 1
	https.Metrics["total_ms"] = stat
	s = replace(s, https)
	got, _ := New(config.Default().Thresholds).Diagnose(s)
	if got.Code != model.DiagnosisUnknown {
		t.Fatalf("got %s", got.Code)
	}
}

func TestCorrectedWSLBaselineReportsInconclusiveDNSRatherThanInsufficientSamples(t *testing.T) {
	summaries := []model.CollectorSummary{
		realSummary("device_resources", "local_device", 60, 60, "load_per_core", .005),
		realSummary("dns_comparison", "1.1.1.1:53/example.com", 30, 8, "dns_ms", 170.0955),
		realSummary("dns_comparison", "8.8.8.8:53/example.com", 30, 29, "dns_ms", 200.518),
		realSummary("dns_system", "example.com", 31, 30, "dns_ms", 174.034),
		realSummary("https", "https://www.cloudflare.com/cdn-cgi/trace", 6, 6, "total_ms", 697.949),
		realSummary("https", "https://www.google.com/generate_204", 6, 6, "total_ms", 792.771),
		realSummary("tcp_connect", "1.1.1.1:443", 30, 30, "connect_ms", 170.4195),
		realSummary("tcp_connect", "8.8.8.8:443", 30, 30, "connect_ms", 198.224),
	}
	device := summaries[0]
	device.Metrics["memory_available_percent"] = model.Statistics{Count: 60, Median: 87.1482}
	summaries[0] = device
	cloudflare := summaries[1]
	cloudflare.Errors = map[model.ErrorCategory]int{model.ErrorTimeout: 22}
	cloudflare.TimeoutRate, cloudflare.LongestFailureRun = 22.0/30, 21
	summaries[1] = cloudflare
	google := summaries[2]
	google.Errors = map[model.ErrorCategory]int{model.ErrorTimeout: 1}
	google.TimeoutRate, google.LongestFailureRun = 1.0/30, 1
	summaries[2] = google

	got, evidence := New(config.Default().Thresholds).Diagnose(summaries)
	if got.Code != model.DiagnosisUnknown {
		t.Fatalf("got %s", got.Code)
	}
	if !hasEvidence(evidence, "slow-dns-without-healthy-contrast") || !hasEvidence(evidence, "repeated-comparison-dns-failures") {
		t.Fatalf("missing session-specific evidence: %#v", evidence)
	}
	if hasEvidence(evidence, "insufficient-evidence") {
		t.Fatalf("all required collectors had enough samples: %#v", evidence)
	}
}

func TestUnexpectedHTTPSStatusIsNotHealthyEndpointEvidence(t *testing.T) {
	s := healthySummaries()
	https := s[4]
	https.TransportSuccesses, https.TransportSuccessRate = 3, 1
	https.EndpointAttempts, https.EndpointSuccesses, https.EndpointSuccessRate = 3, 0, 0
	https.Metrics["status_code"] = model.Statistics{Count: 3, Median: 404}
	s = replace(s, https)
	got, evidence := New(config.Default().Thresholds).Diagnose(s)
	if got.Code != model.DiagnosisUnknown || !hasEvidence(evidence, "unexpected-endpoint-response") {
		t.Fatalf("unexpected diagnosis: %#v %#v", got, evidence)
	}
	if https.Metrics["total_ms"].Count != 3 {
		t.Fatal("timing evidence was not retained")
	}
}

func realSummary(kind, target string, attempts, successes int, metric string, median float64) model.CollectorSummary {
	failureRate := float64(attempts-successes) / float64(attempts)
	return model.CollectorSummary{
		CollectorType: kind,
		Target:        target,
		Attempts:      attempts,
		Successes:     successes,
		SuccessRate:   float64(successes) / float64(attempts),
		FailureRate:   failureRate,
		Metrics:       map[string]model.Statistics{metric: {Count: successes, Median: median}},
	}
}

func hasEvidence(evidence []model.Evidence, id string) bool {
	for _, item := range evidence {
		if item.ID == id {
			return true
		}
	}
	return false
}

func healthySummaries() []model.CollectorSummary {
	return []model.CollectorSummary{summary("dns_system", "example.com", "dns_ms", 20), summary("dns_comparison", "1.1.1.1:53/example.com", "dns_ms", 25), summary("tcp_connect", "slow:443", "connect_ms", 40), summary("tcp_connect", "fast:443", "connect_ms", 45), summary("https", "https://example.com", "total_ms", 200), device(.2, 50)}
}
func summary(kind, target, metric string, median float64) model.CollectorSummary {
	return model.CollectorSummary{CollectorType: kind, Target: target, Attempts: 3, Successes: 3, Metrics: map[string]model.Statistics{metric: {Count: 3, Min: median, Median: median, Max: median, Mean: median}}}
}
func oneSample(kind, target, metric string, value float64) model.CollectorSummary {
	s := summary(kind, target, metric, value)
	s.Attempts, s.Successes = 1, 1
	stat := s.Metrics[metric]
	stat.Count = 1
	s.Metrics[metric] = stat
	return s
}
func device(load, memory float64) model.CollectorSummary {
	s := summary("device_resources", "local_device", "load_per_core", load)
	s.Metrics["memory_available_percent"] = model.Statistics{Count: 3, Min: memory, Median: memory, Max: memory, Mean: memory}
	return s
}
func replace(all []model.CollectorSummary, replacement model.CollectorSummary) []model.CollectorSummary {
	out := append([]model.CollectorSummary{}, all...)
	for i, item := range out {
		if item.CollectorType == replacement.CollectorType && item.Target == replacement.Target {
			out[i] = replacement
			return out
		}
	}
	return append(out, replacement)
}
