package webapp

import "whyfi/internal/model"

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
	set("device", summaryStatus(byType["device_resources"]))
	local := summaryStatus(byType["wifi"])
	if local == "unavailable" {
		for _, gateway := range byType["gateway"] {
			if gateway.DetailCounts["connection_kind=ethernet"] > 0 && gateway.Successes > 0 {
				local = "healthy"
			}
		}
	}
	set("local", local)
	set("gateway", summaryStatus(byType["gateway"]))
	dns := append(append([]model.CollectorSummary{}, byType["dns_system"]...), byType["dns_comparison"]...)
	set("internet", summaryStatus(dns))
	destinations := append(append([]model.CollectorSummary{}, byType["tcp_connect"]...), byType["https"]...)
	set("destinations", summaryStatus(destinations))
	switch session.Diagnosis.Code {
	case model.DiagnosisDevice:
		set("device", "degraded")
	case model.DiagnosisLocalWiFi:
		set("local", "degraded")
		set("gateway", "degraded")
	case model.DiagnosisGateway:
		set("gateway", "degraded")
	case model.DiagnosisDNS:
		set("internet", "degraded")
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
			if layers[i].Status != "unavailable" {
				layers[i].Status = "inconclusive"
			}
		}
	}
	return layers
}

func summaryStatus(summaries []model.CollectorSummary) string {
	if len(summaries) == 0 {
		return "unavailable"
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
	if unsupportedOnly || !hasSuccess {
		return "unavailable"
	}
	if degraded {
		return "degraded"
	}
	return "healthy"
}
