package diagnosis

import (
	"testing"

	"whyfi/internal/config"
	"whyfi/internal/model"
)

func TestMilestone15LocalVersusUpstreamRules(t *testing.T) {
	tests := []struct {
		name      string
		summaries []model.CollectorSummary
		want      model.DiagnosisCode
	}{
		{"strong stable Wi-Fi and gateway", withLocal(healthySummaries(), stableWiFi(), stableGateway("wifi")), model.DiagnosisHealthy},
		{"weak fluctuating Wi-Fi and unstable gateway", withLocal(healthySummaries(), weakWiFi(), unstableGateway()), model.DiagnosisLocalWiFi},
		{"Wi-Fi reconnect and unstable gateway", withLocal(healthySummaries(), reconnectingWiFi(), unstableGateway()), model.DiagnosisLocalWiFi},
		{"stable Wi-Fi but unstable gateway", withLocal(healthySummaries(), stableWiFi(), unstableGateway()), model.DiagnosisGateway},
		{"stable local path and two degraded public targets", withLocal(twoSlowTargets(healthySummaries()), stableWiFi(), stableGateway("wifi")), model.DiagnosisUpstream},
		{"one degraded destination only", withLocal(replace(healthySummaries(), summary("tcp_connect", "slow:443", "connect_ms", 400)), stableWiFi(), stableGateway("wifi")), model.DiagnosisDestination},
		{"ICMP unavailable", withLocal(healthySummaries(), stableWiFi(), gatewayError(model.ErrorICMPBlocked)), model.DiagnosisUnknown},
		{"no Wi-Fi adapter on Wi-Fi route", withLocal(healthySummaries(), wifiError(model.ErrorNoAdapter), stableGateway("wifi")), model.DiagnosisUnknown},
		{"Ethernet connection", withLocal(twoSlowTargets(healthySummaries()), wifiError(model.ErrorNoAdapter), stableGateway("ethernet")), model.DiagnosisUpstream},
		{"VPN default route", withLocal(healthySummaries(), stableWiFi(), stableGateway("vpn_or_tunnel")), model.DiagnosisUnknown},
		{"multiple active adapters", withLocal(healthySummaries(), stableWiFi(), gatewayError(model.ErrorAmbiguous)), model.DiagnosisUnknown},
		{"conflicting weak Wi-Fi and stable gateway", withLocal(twoSlowTargets(healthySummaries()), weakWiFi(), stableGateway("wifi")), model.DiagnosisUnknown},
	}
	engine := New(config.Default().Thresholds)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := engine.Diagnose(test.summaries)
			if got.Code != test.want {
				t.Fatalf("got %s, want %s: %#v", got.Code, test.want, got)
			}
		})
	}
}

func TestUpstreamRequiresTwoIndependentPublicTargets(t *testing.T) {
	s := healthySummaries()
	s = replace(s, summary("https", "https://only.example", "total_ms", 2000))
	got, _ := New(config.Default().Thresholds).Diagnose(withLocal(s, stableWiFi(), stableGateway("wifi")))
	if got.Code == model.DiagnosisUpstream {
		t.Fatal("one public endpoint produced upstream diagnosis")
	}
}

func stableWiFi() model.CollectorSummary {
	return localSummary("wifi", "Wi-Fi network 1", map[string]model.Statistics{
		"connected": {Count: 3, Median: 1, Max: 1}, "signal_quality_percent": {Count: 3, Median: 80, MAD: 2, Max: 82}, "state_change": {Count: 3}, "reconnect_count": {Count: 3},
	})
}
func weakWiFi() model.CollectorSummary {
	return localSummary("wifi", "Wi-Fi network 1", map[string]model.Statistics{
		"connected": {Count: 3, Median: 1, Max: 1}, "signal_quality_percent": {Count: 3, Median: 28, MAD: 12, Max: 55}, "state_change": {Count: 3}, "reconnect_count": {Count: 3},
	})
}
func reconnectingWiFi() model.CollectorSummary {
	s := stableWiFi()
	s.Metrics["state_change"] = model.Statistics{Count: 3, Max: 1}
	s.Metrics["reconnect_count"] = model.Statistics{Count: 3, Max: 1}
	return s
}
func stableGateway(kind string) model.CollectorSummary {
	s := localSummary("gateway", "local_gateway", map[string]model.Statistics{"gateway_ms": {Count: 3, Median: 5, Max: 7}})
	s.DetailCounts = map[string]int{"connection_kind=" + kind: 3, "reachability_method=windows_icmp": 3}
	return s
}
func unstableGateway() model.CollectorSummary {
	s := localSummary("gateway", "local_gateway", map[string]model.Statistics{"gateway_ms": {Count: 7, Median: 90, Max: 200}})
	s.Attempts, s.Successes, s.SuccessRate, s.FailureRate, s.TimeoutRate, s.LongestFailureRun = 10, 7, .7, .3, .3, 2
	s.Errors = map[model.ErrorCategory]int{model.ErrorTimeout: 3}
	s.DetailCounts = map[string]int{"connection_kind=wifi": 10}
	return s
}
func gatewayError(category model.ErrorCategory) model.CollectorSummary {
	return model.CollectorSummary{CollectorType: "gateway", Target: "local_gateway", Attempts: 3, FailureRate: 1, LongestFailureRun: 3, Errors: map[model.ErrorCategory]int{category: 3}}
}
func wifiError(category model.ErrorCategory) model.CollectorSummary {
	return model.CollectorSummary{CollectorType: "wifi", Target: "Wi-Fi network 1", Attempts: 3, FailureRate: 1, LongestFailureRun: 3, Errors: map[model.ErrorCategory]int{category: 3}}
}
func localSummary(kind, target string, metrics map[string]model.Statistics) model.CollectorSummary {
	return model.CollectorSummary{CollectorType: kind, Target: target, Attempts: 3, Successes: 3, SuccessRate: 1, Metrics: metrics}
}
func withLocal(s []model.CollectorSummary, wifi, gateway model.CollectorSummary) []model.CollectorSummary {
	return append(append(append([]model.CollectorSummary{}, s...), wifi), gateway)
}
func twoSlowTargets(s []model.CollectorSummary) []model.CollectorSummary {
	s = replace(s, summary("tcp_connect", "slow:443", "connect_ms", 400))
	return replace(s, summary("tcp_connect", "fast:443", "connect_ms", 420))
}
