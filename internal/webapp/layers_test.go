package webapp

import (
	"testing"

	"whyfi/internal/model"
)

func TestUnsupportedDeviceResourcesAreNotMeasured(t *testing.T) {
	session := model.DiagnosticSession{Summaries: []model.CollectorSummary{{CollectorType: "device_resources", Target: "local_device", Attempts: 1, FailureRate: 1, Errors: map[model.ErrorCategory]int{model.ErrorUnsupported: 1}}}}
	layers := finalLayers(session)
	if len(layers) != 5 || layers[0].ID != "local" {
		t.Fatalf("snapshot layers = %#v, want five network layers without device resources", layers)
	}
}

func TestNativeHealthyShapeProducesFactualCards(t *testing.T) {
	p95 := 28.0
	session := model.DiagnosticSession{Summaries: []model.CollectorSummary{
		{CollectorType: "device_resources", Attempts: 1, Errors: map[model.ErrorCategory]int{model.ErrorUnsupported: 1}, FailureRate: 1},
		{CollectorType: "wifi", Attempts: 60, Successes: 60, Metrics: map[string]model.Statistics{"signal_quality_percent": {Count: 60, Median: 82}}},
		{CollectorType: "gateway", Attempts: 60, Successes: 60, Metrics: map[string]model.Statistics{"gateway_ms": {Count: 60, Median: 4, P95: &p95, OutlierCount: 3}}},
		{CollectorType: "dns_system", Attempts: 30, Successes: 27, Errors: map[model.ErrorCategory]int{model.ErrorTimeout: 3}},
		{CollectorType: "tcp_connect", Attempts: 20, Successes: 20},
		{CollectorType: "https", Attempts: 8, Successes: 8},
	}}
	layers := finalLayers(session)
	if layers[1].Observation != "Gateway median latency was 4 ms with no meaningful loss." {
		t.Fatalf("gateway observation = %q", layers[1].Observation)
	}
	if layers[2].Observation != "3 of 30 DNS checks timed out." {
		t.Fatalf("DNS observation = %q", layers[2].Observation)
	}
	for _, layer := range layers {
		if layer.Status != "healthy" {
			t.Fatalf("layer %#v should be healthy", layer)
		}
	}
}
