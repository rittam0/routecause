package webapp

import (
	"encoding/json"
	"strings"

	"whyfi/internal/model"
)

func sanitizeSession(input model.DiagnosticSession) model.DiagnosticSession {
	data, _ := json.Marshal(input)
	var output model.DiagnosticSession
	_ = json.Unmarshal(data, &output)
	for i := range output.RawResults {
		r := &output.RawResults[i]
		if r.CollectorType == "wifi" {
			r.Target = "Wi-Fi network 1"
		}
		if r.CollectorType == "gateway" {
			r.Target = "local_gateway"
		}
		safe := map[string]string{}
		for key, value := range r.Details {
			switch key {
			case "network_label":
				safe[key] = "Wi-Fi network 1"
			case "adapter_label":
				safe[key] = "Wi-Fi adapter 1"
			case "radio", "connection_kind", "gateway_label", "reachability_method":
				if !containsPrivateMarker(value) {
					safe[key] = value
				}
			}
		}
		r.Details = safe
	}
	for i := range output.Summaries {
		if output.Summaries[i].CollectorType == "wifi" {
			output.Summaries[i].Target = "Wi-Fi network 1"
		}
	}
	return output
}

func containsPrivateMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"ssid", "bssid", "guid", "mac", "gateway_address"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
