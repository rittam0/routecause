package webapp

import (
	"context"
	"time"

	"whyfi/internal/model"
)

type DemoRunner struct{}

var demoCodes = map[string]model.DiagnosisCode{
	"healthy":     model.DiagnosisHealthy,
	"dns":         model.DiagnosisDNS,
	"wifi":        model.DiagnosisLocalWiFi,
	"gateway":     model.DiagnosisGateway,
	"upstream":    model.DiagnosisUpstream,
	"destination": model.DiagnosisDestination,
	"unknown":     model.DiagnosisUnknown,
}

func validDemoScenario(value string) bool { _, ok := demoCodes[value]; return ok }

func (DemoRunner) Run(ctx context.Context, _ time.Duration, observer func(model.ProbeResult)) (model.DiagnosticSession, error) {
	scenario, _ := ctx.Value(demoScenarioKey{}).(string)
	if !validDemoScenario(scenario) {
		scenario = "healthy"
	}
	started := time.Now()
	collectorTypes := []string{"device_resources", "wifi", "gateway", "dns_system", "tcp_connect", "https"}
	raw := make([]model.ProbeResult, 0, len(collectorTypes))
	for i, kind := range collectorTypes {
		select {
		case <-ctx.Done():
			return model.DiagnosticSession{}, ctx.Err()
		case <-time.After(120 * time.Millisecond):
		}
		result := model.ProbeResult{ID: "demo", CollectorType: kind, Target: demoTarget(kind), StartedAt: started.Add(time.Duration(i) * 120 * time.Millisecond), Success: true, Values: map[string]float64{"demo_sample": 1}}
		raw = append(raw, result)
		observer(result)
	}
	return demoSession(scenario, started, raw), nil
}

type demoScenarioKey struct{}

type scenarioRunner struct {
	scenario string
	runner   Runner
}

func (r scenarioRunner) Run(ctx context.Context, duration time.Duration, observer func(model.ProbeResult)) (model.DiagnosticSession, error) {
	return r.runner.Run(context.WithValue(ctx, demoScenarioKey{}, r.scenario), duration, observer)
}

func demoTarget(kind string) string {
	if kind == "wifi" {
		return "Wi-Fi network 1"
	}
	if kind == "gateway" {
		return "local_gateway"
	}
	return "demo_target"
}

func demoSession(scenario string, started time.Time, raw []model.ProbeResult) model.DiagnosticSession {
	code := demoCodes[scenario]
	conclusions := map[model.DiagnosisCode]string{
		model.DiagnosisHealthy:     "No material problem was observed during this demo session.",
		model.DiagnosisDNS:         "The system DNS path is likely degraded relative to comparison resolvers.",
		model.DiagnosisLocalWiFi:   "Physical Wi-Fi instability is a likely local contributor.",
		model.DiagnosisGateway:     "The local gateway or local network path is likely unstable.",
		model.DiagnosisUpstream:    "Degradation is likely beyond the stable local network path.",
		model.DiagnosisDestination: "Degradation appears specific to one measured destination.",
		model.DiagnosisUnknown:     "There is insufficient or contradictory evidence to identify one cause.",
	}
	confidence := "moderate"
	if code == model.DiagnosisUnknown {
		confidence = ""
	}
	evidence := []model.Evidence{{ID: "demo-evidence", Statement: "RouteCause deterministic demonstration measurements support this example result.", Supports: true, SummaryKeys: []string{"demo"}}}
	summaries := []model.CollectorSummary{}
	for _, result := range raw {
		summaries = append(summaries, model.CollectorSummary{CollectorType: result.CollectorType, Target: result.Target, Attempts: 3, Successes: 3, SuccessRate: 1, Metrics: map[string]model.Statistics{"demo_ms": {Count: 3, Min: 10, Median: 10, Max: 10, Mean: 10}}})
	}
	return model.DiagnosticSession{SchemaVersion: model.SchemaVersion, ID: "demo-" + scenario, StartedAt: started, EndedAt: time.Now(), RawResults: raw, Summaries: summaries, Evidence: evidence, Diagnosis: model.Diagnosis{Code: code, Conclusion: conclusions[code], Confidence: confidence, SupportingEvidence: evidence, ContradictingEvidence: []model.Evidence{}, Alternatives: []string{"This is a deterministic demo, not a measurement of the current connection."}}, Limitations: []string{"ROUTECAUSE INTERACTIVE DEMO MODE: no live network or device measurements were taken."}}
}
