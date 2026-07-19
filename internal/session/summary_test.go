package session

import (
	"math"
	"testing"
	"time"

	"whyfi/internal/model"
)

func TestSummarizeSeparatesFailuresAndComputesMedian(t *testing.T) {
	results := []model.ProbeResult{{CollectorType: "tcp_connect", Target: "a:443", StartedAt: time.Unix(1, 0), Success: true, Values: map[string]float64{"connect_ms": 30}}, {CollectorType: "tcp_connect", Target: "a:443", StartedAt: time.Unix(2, 0), Success: true, Values: map[string]float64{"connect_ms": 10}}, {CollectorType: "tcp_connect", Target: "a:443", StartedAt: time.Unix(3, 0), ErrorCategory: model.ErrorTimeout, Values: map[string]float64{"connect_ms": 1000}}}
	got := Summarize(results)
	if len(got) != 1 || got[0].Attempts != 3 || got[0].Successes != 2 || got[0].Errors[model.ErrorTimeout] != 1 {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if got[0].Metrics["connect_ms"].Median != 20 {
		t.Fatalf("median = %v", got[0].Metrics["connect_ms"].Median)
	}
	if math.Abs(got[0].SuccessRate-2.0/3) > 1e-12 || math.Abs(got[0].FailureRate-1.0/3) > 1e-12 || math.Abs(got[0].TimeoutRate-1.0/3) > 1e-12 || got[0].LongestFailureRun != 1 {
		t.Fatalf("unexpected reliability statistics: %#v", got[0])
	}
}

func TestPercentilesMADAndRobustOutliers(t *testing.T) {
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(10 + i%5)
	}
	values[99] = 1000
	got := statistics(values)
	if got.P95 == nil || *got.P95 != 14 || got.P99 == nil || *got.P99 != 14 {
		t.Fatalf("unexpected percentiles: %#v", got)
	}
	if got.MAD != 1 || got.OutlierCount != 1 {
		t.Fatalf("unexpected robust statistics: %#v", got)
	}
}

func TestLongestConsecutiveFailures(t *testing.T) {
	results := []model.ProbeResult{
		{StartedAt: time.Unix(1, 0), Success: true},
		{StartedAt: time.Unix(2, 0)},
		{StartedAt: time.Unix(3, 0)},
		{StartedAt: time.Unix(4, 0), Success: true},
	}
	if got := longestFailureRun(results); got != 2 {
		t.Fatalf("got %d", got)
	}
}

func TestHTTPSTransportAndEndpointRatesAreSeparate(t *testing.T) {
	transport, expected, unexpected := true, true, false
	results := []model.ProbeResult{
		{CollectorType: "https", Target: "health", Success: true, TransportSuccess: &transport, EndpointSuccess: &expected, Values: map[string]float64{"total_ms": 10, "status_code": 200}},
		{CollectorType: "https", Target: "health", Success: true, TransportSuccess: &transport, EndpointSuccess: &unexpected, Values: map[string]float64{"total_ms": 12, "status_code": 404}},
	}
	got := Summarize(results)[0]
	if got.TransportSuccesses != 2 || got.TransportSuccessRate != 1 || got.EndpointAttempts != 2 || got.EndpointSuccesses != 1 || got.EndpointSuccessRate != .5 {
		t.Fatalf("unexpected HTTPS rates: %#v", got)
	}
	if got.Metrics["total_ms"].Count != 2 {
		t.Fatal("unexpected response lost timing evidence")
	}
}

func TestGatewayLossExcludesRouteOnlyICMPUnsupported(t *testing.T) {
	results := []model.ProbeResult{
		{CollectorType: "gateway", Target: "local_gateway", Success: true, Values: map[string]float64{"default_route_present": 1, "gateway_ms": 5}},
		{CollectorType: "gateway", Target: "local_gateway", ErrorCategory: model.ErrorTimeout, Values: map[string]float64{"default_route_present": 1}},
		{CollectorType: "gateway", Target: "local_gateway", ErrorCategory: model.ErrorICMPBlocked, Values: map[string]float64{"default_route_present": 1}},
	}
	got := Summarize(results)[0]
	if got.Losses != 1 || math.Abs(got.LossRate-1.0/3) > 1e-12 {
		t.Fatalf("unexpected gateway loss: %#v", got)
	}
}
