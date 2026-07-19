package webapp

import (
	"fmt"

	"whyfi/internal/model"
)

func finalLayers(session model.DiagnosticSession) []LayerState {
	layers := cloneLayers(layerOrder)
	if session.ID == "demo-unknown" {
		for i := range layers {
			layers[i].Status = "inconclusive"
		}
		return layers
	}
	byType := map[string][]model.CollectorSummary{}
	for _, summary := range session.Summaries {
		byType[summary.CollectorType] = append(byType[summary.CollectorType], summary)
	}
	set := func(id, status string) {
		for i := range layers {
			if layers[i].ID == id {
				layers[i].Status = status
			}
		}
	}
	local := summaryStatus(byType["wifi"])
	if local == "unavailable" || local == "not_measured" {
		for _, gateway := range byType["gateway"] {
			if gateway.DetailCounts["connection_kind=ethernet"] > 0 && gateway.Successes > 0 {
				local = "healthy"
			}
		}
	}
	set("local", local)
	set("gateway", summaryStatus(byType["gateway"]))
	dns := append(append([]model.CollectorSummary{}, byType["dns_system"]...), byType["dns_comparison"]...)
	set("dns", summaryStatus(dns))
	set("internet", summaryStatus(byType["tcp_connect"]))
	set("destinations", summaryStatus(byType["https"]))
	switch session.Diagnosis.Code {
	case model.DiagnosisDevice:
		set("local", "degraded")
	case model.DiagnosisLocalWiFi:
		set("local", "degraded")
		set("gateway", "degraded")
	case model.DiagnosisGateway:
		set("gateway", "degraded")
	case model.DiagnosisDNS:
		set("dns", "degraded")
	case model.DiagnosisUpstream:
		set("internet", "degraded")
		set("destinations", "degraded")
	case model.DiagnosisDestination:
		set("destinations", "degraded")
	case model.DiagnosisUnknown:
		for i := range layers {
			if layers[i].Status == "healthy" {
				continue
			}
			if layers[i].Status != "unavailable" && layers[i].Status != "not_measured" {
				layers[i].Status = "inconclusive"
			}
		}
	}
	for i := range layers {
		layers[i].Observation = layerObservation(layers[i].ID, byType)
	}
	return layers
}

func layerObservation(id string, byType map[string][]model.CollectorSummary) string {
	switch id {
	case "local":
		if len(byType["wifi"]) == 0 {
			return "Wi-Fi details were not measured; an Ethernet route may still have been observed."
		}
		s := byType["wifi"][0]
		if q, ok := s.Metrics["signal_quality_percent"]; ok {
			return fmt.Sprintf("Wi-Fi signal quality had a median of %.0f%% across %d checks.", q.Median, s.Attempts)
		}
		return fmt.Sprintf("The local connection was checked %d times.", s.Attempts)
	case "gateway":
		if len(byType["gateway"]) == 0 {
			return "Gateway measurements were not collected."
		}
		s := byType["gateway"][0]
		if stat, ok := s.Metrics["gateway_ms"]; ok {
			if s.LossRate == 0 {
				return fmt.Sprintf("Gateway median latency was %.0f ms with no meaningful loss.", stat.Median)
			}
			return fmt.Sprintf("Gateway median latency was %.0f ms with %.1f%% loss.", stat.Median, 100*s.LossRate)
		}
		return fmt.Sprintf("Gateway checks completed %d of %d attempts.", s.Successes, s.Attempts)
	case "dns":
		all := append(append([]model.CollectorSummary{}, byType["dns_system"]...), byType["dns_comparison"]...)
		attempts, timeouts := 0, 0
		for _, s := range all {
			attempts += s.Attempts
			timeouts += s.Errors[model.ErrorTimeout]
		}
		if attempts == 0 {
			return "DNS measurements were not collected."
		}
		return fmt.Sprintf("%d of %d DNS checks timed out.", timeouts, attempts)
	case "internet":
		return successObservation("Internet connection", byType["tcp_connect"])
	case "destinations":
		return successObservation("Destination", byType["https"])
	}
	return ""
}

func successObservation(label string, summaries []model.CollectorSummary) string {
	attempts, successes := 0, 0
	for _, s := range summaries {
		attempts += s.Attempts
		successes += s.Successes
	}
	if attempts == 0 {
		return label + " measurements were not collected."
	}
	return fmt.Sprintf("%s checks completed %d of %d attempts.", label, successes, attempts)
}

func summaryStatus(summaries []model.CollectorSummary) string {
	if len(summaries) == 0 {
		return "inconclusive"
	}
	hasSuccess, degraded, unsupportedOnly := false, false, true
	for _, summary := range summaries {
		if summary.Successes > 0 {
			hasSuccess, unsupportedOnly = true, false
		}
		if summary.Errors[model.ErrorUnsupported] < summary.Attempts {
			unsupportedOnly = false
		}
		if summary.FailureRate > .1 || summary.LongestFailureRun > 1 {
			degraded = true
		}
	}
	if unsupportedOnly {
		return "not_measured"
	}
	if !hasSuccess {
		return "inconclusive"
	}
	if degraded {
		return "degraded"
	}
	return "healthy"
}
