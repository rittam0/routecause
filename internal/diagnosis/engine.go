package diagnosis

import (
	"fmt"
	"sort"
	"strings"

	"whyfi/internal/config"
	"whyfi/internal/model"
)

type Engine struct{ thresholds config.Thresholds }

func New(thresholds config.Thresholds) Engine { return Engine{thresholds: thresholds} }

type candidate struct {
	code                      model.DiagnosisCode
	conclusion                string
	supporting, contradicting []model.Evidence
	alternatives              []string
}

func (e Engine) Diagnose(summaries []model.CollectorSummary) (model.Diagnosis, []model.Evidence) {
	byType := map[string][]model.CollectorSummary{}
	for _, summary := range summaries {
		byType[summary.CollectorType] = append(byType[summary.CollectorType], summary)
	}
	if nativeLocalEvidence(byType) {
		localCandidates, localEvidence, localReady := e.localCandidates(byType)
		if c, ok := e.deviceCandidate(byType["device_resources"]); ok {
			localCandidates = append(localCandidates, c)
		}
		if c, ok := e.dnsCandidate(byType["dns_system"], byType["dns_comparison"]); ok {
			localCandidates = append(localCandidates, c)
		}
		if len(localCandidates) > 1 {
			evidence := append([]model.Evidence{{ID: "conflicting-signals", Statement: "Multiple local, upstream, destination, or device degradation patterns were observed; one cause cannot be isolated.", Supports: false}}, localEvidence...)
			for _, c := range localCandidates {
				evidence = append(evidence, c.supporting...)
			}
			return unknown("Conflicting signals prevent an honest preliminary conclusion.", evidence), evidence
		}
		if len(localCandidates) == 1 {
			c := localCandidates[0]
			all := append(append([]model.Evidence{}, c.supporting...), c.contradicting...)
			return model.Diagnosis{Code: c.code, Conclusion: c.conclusion, Confidence: "moderate", SupportingEvidence: evidenceSlice(c.supporting), ContradictingEvidence: evidenceSlice(c.contradicting), Alternatives: stringSlice(c.alternatives)}, all
		}
		if !localReady {
			if len(localEvidence) == 0 {
				localEvidence = []model.Evidence{{ID: "insufficient-local-evidence", Statement: "Native Wi-Fi/default-route or gateway measurements were missing, unsupported, or ambiguous.", Supports: true, SummaryKeys: append(keys(byType["wifi"]), keys(byType["gateway"])...)}}
			}
			return unknown("There is insufficient or contradictory local-versus-upstream evidence.", localEvidence), localEvidence
		}
	}
	candidates := []candidate{}
	if c, ok := e.deviceCandidate(byType["device_resources"]); ok {
		candidates = append(candidates, c)
	}
	if c, ok := e.dnsCandidate(byType["dns_system"], byType["dns_comparison"]); ok {
		candidates = append(candidates, c)
	}
	if c, ok := e.destinationCandidate(byType["tcp_connect"]); ok {
		candidates = append(candidates, c)
	}
	if len(candidates) > 1 {
		evidence := []model.Evidence{{ID: "conflicting-signals", Statement: "Multiple milestone-one degradation patterns were observed; the available measurements cannot isolate one cause.", Supports: false, SummaryKeys: []string{}}}
		for _, c := range candidates {
			evidence = append(evidence, c.supporting...)
		}
		return unknown("Conflicting signals prevent an honest preliminary conclusion.", evidence), evidence
	}
	if len(candidates) == 1 {
		c := candidates[0]
		all := append(append([]model.Evidence{}, c.supporting...), c.contradicting...)
		return model.Diagnosis{Code: c.code, Conclusion: c.conclusion, Confidence: "moderate", SupportingEvidence: evidenceSlice(c.supporting), ContradictingEvidence: evidenceSlice(c.contradicting), Alternatives: stringSlice(c.alternatives)}, all
	}
	healthy, issues := e.healthyAssessment(byType)
	if healthy {
		ev := []model.Evidence{{ID: "typical-performance-within-thresholds", Statement: "Typical DNS, TCP, HTTPS, and device resource measurements remained within configured thresholds.", Supports: true, SummaryKeys: keys(summaries)}}
		if unstable := instabilityEvidence(summaries); unstable != nil {
			ev = append(ev, *unstable)
		}
		return model.Diagnosis{Code: model.DiagnosisHealthy, Conclusion: "No material problem was observed during this session.", Confidence: "moderate", SupportingEvidence: ev, ContradictingEvidence: []model.Evidence{}, Alternatives: []string{"An intermittent issue may not have occurred during the measurement window.", "Native Wi-Fi and gateway evidence is available only when running directly on Windows."}}, ev
	}
	issues = append(issues, e.inconclusivePatternEvidence(byType)...)
	ev := issues
	if len(ev) == 0 {
		ev = []model.Evidence{{ID: "insufficient-evidence", Statement: fmt.Sprintf("The available summaries did not meet the minimum of %d successful samples for a supported conclusion.", e.thresholds.MinimumSamples), Supports: true, SummaryKeys: keys(summaries)}}
	}
	return unknown("There is insufficient or inconclusive evidence to identify a supported cause.", ev), ev
}

func nativeLocalEvidence(byType map[string][]model.CollectorSummary) bool {
	for _, summary := range append(append([]model.CollectorSummary{}, byType["wifi"]...), byType["gateway"]...) {
		if summary.Successes > 0 || summary.Errors[model.ErrorNoAdapter] > 0 || summary.Errors[model.ErrorDisconnected] > 0 || summary.Errors[model.ErrorAmbiguous] > 0 || summary.Errors[model.ErrorICMPBlocked] > 0 {
			return true
		}
	}
	return false
}

func (e Engine) localCandidates(byType map[string][]model.CollectorSummary) ([]candidate, []model.Evidence, bool) {
	wifi, gateway := byType["wifi"], byType["gateway"]
	evidence := []model.Evidence{}
	if len(gateway) != 1 {
		return nil, evidence, false
	}
	g := gateway[0]
	if g.Errors[model.ErrorAmbiguous] > 0 {
		return nil, []model.Evidence{{ID: "ambiguous-default-route", Statement: "Multiple equally preferred default routes were observed; local-path attribution is unsafe.", Supports: true, SummaryKeys: []string{key(g)}}}, false
	}
	if g.Errors[model.ErrorICMPBlocked] > 0 && !metricAdequate(g, "gateway_ms", e.thresholds.MinimumSamples) {
		return nil, []model.Evidence{{ID: "gateway-icmp-unavailable", Statement: "The IPv4 default route was found, but unprivileged Windows ICMP gateway measurements were unavailable; route presence is not treated as latency or reachability.", Supports: true, SummaryKeys: []string{key(g)}}}, false
	}
	gatewayStable := adequate(g, "gateway_ms", e.thresholds.MinimumSamples) && g.Metrics["gateway_ms"].Median < e.thresholds.GatewaySlowMS && (g.Metrics["gateway_ms"].P95 == nil || *g.Metrics["gateway_ms"].P95 < e.thresholds.GatewaySlowMS)
	gatewayUnstable := metricAdequate(g, "gateway_ms", e.thresholds.MinimumSamples) && (g.FailureRate > .10 || g.LongestFailureRun > 1 || g.Metrics["gateway_ms"].Median >= e.thresholds.GatewaySlowMS || (g.Metrics["gateway_ms"].P95 != nil && *g.Metrics["gateway_ms"].P95 >= e.thresholds.GatewaySlowMS))

	wifiStable, wifiUnstable := false, false
	if len(wifi) == 1 && metricAdequate(wifi[0], "connected", e.thresholds.MinimumSamples) {
		w := wifi[0]
		connected := w.Metrics["connected"].Median >= 1
		reconnects := metricMax(w, "reconnect_count")
		changes := metricMax(w, "state_change")
		signal, hasSignal := w.Metrics["signal_quality_percent"]
		wifiStable = connected && hasSignal && signal.Median >= e.thresholds.WiFiWeakPercent && reconnects == 0 && changes == 0
		wifiUnstable = !connected || reconnects > 0 || changes > 0 || (hasSignal && (signal.Median < e.thresholds.WiFiWeakPercent || signal.MAD > 10))
	}
	ethernet := g.DetailCounts["connection_kind=ethernet"] > 0
	wifiRoute := g.DetailCounts["connection_kind=wifi"] > 0
	vpn := g.DetailCounts["connection_kind=vpn_or_tunnel"] > 0
	if vpn {
		return nil, []model.Evidence{{ID: "vpn-route-ambiguity", Statement: "The active default route is a VPN or tunnel, so physical local-path attribution is withheld.", Supports: true, SummaryKeys: []string{key(g)}}}, false
	}
	localStable := gatewayStable && ((wifiRoute && wifiStable) || ethernet)

	if wifiRoute && wifiUnstable && gatewayUnstable {
		support := []model.Evidence{{ID: "unstable-wifi", Statement: "Native Wi-Fi measurements showed weak/fluctuating signal, disconnection, or reconnect activity.", Supports: true, SummaryKeys: keys(wifi)}, {ID: "unstable-gateway", Statement: "Local gateway latency or loss was unstable during the same session.", Supports: true, SummaryKeys: []string{key(g)}}}
		return []candidate{{code: model.DiagnosisLocalWiFi, conclusion: "Physical Wi-Fi instability is a likely local contributor.", supporting: support, alternatives: []string{"Radio interference, access-point behavior, or adapter/driver behavior may produce similar signals.", "Another upstream issue may coexist."}}}, evidence, true
	}
	if wifiRoute && wifiStable && gatewayUnstable {
		support := []model.Evidence{{ID: "stable-wifi", Statement: "Native Wi-Fi remained connected with acceptable signal and no reconnects.", Supports: true, SummaryKeys: keys(wifi)}, {ID: "unstable-gateway", Statement: "Local gateway latency or loss was unstable.", Supports: true, SummaryKeys: []string{key(g)}}}
		return []candidate{{code: model.DiagnosisGateway, conclusion: "The local gateway or local network path is likely unstable.", supporting: support, alternatives: []string{"Access-point forwarding, router load, or local wired backhaul may contribute.", "ICMP handling by the gateway may differ from application traffic."}}}, evidence, true
	}
	degraded, healthy := publicTargetGroups(byType, e.thresholds)
	if localStable && len(degraded) >= 2 {
		support := []model.Evidence{{ID: "stable-local-path", Statement: "Native Wi-Fi or Ethernet and local gateway measurements remained stable.", Supports: true, SummaryKeys: append(keys(wifi), key(g))}, {ID: "multiple-degraded-public-targets", Statement: "At least two independent configured public targets showed sustained degradation.", Supports: true, SummaryKeys: keys(degraded)}}
		return []candidate{{code: model.DiagnosisUpstream, conclusion: "Degradation is likely beyond the stable local network path.", supporting: support, alternatives: []string{"Regional routing, shared access-network congestion, or independent destination behavior may contribute.", "RouteCause does not attribute this signal to a specific ISP."}}}, evidence, true
	}
	if localStable && len(degraded) == 1 && len(healthy) > 0 {
		support := []model.Evidence{{ID: "stable-local-path", Statement: "Native Wi-Fi or Ethernet and local gateway measurements remained stable.", Supports: true, SummaryKeys: append(keys(wifi), key(g))}, {ID: "one-degraded-destination", Statement: "One configured destination was degraded while an independent alternative remained healthy.", Supports: true, SummaryKeys: append(keys(degraded), keys(healthy)...)}}
		return []candidate{{code: model.DiagnosisDestination, conclusion: "Degradation appears specific to one measured destination.", supporting: support, alternatives: []string{"Destination service behavior or route-specific congestion may be responsible.", "The condition may be transient."}}}, evidence, true
	}
	if localStable && len(degraded) == 0 && len(healthy) >= 2 {
		support := []model.Evidence{{ID: "stable-local-path", Statement: "Native Wi-Fi or Ethernet and local gateway measurements remained stable.", Supports: true, SummaryKeys: append(keys(wifi), key(g))}, {ID: "healthy-independent-targets", Statement: "At least two independently operated configured public targets remained within thresholds.", Supports: true, SummaryKeys: keys(healthy)}}
		return []candidate{{code: model.DiagnosisHealthy, conclusion: "No material problem was observed during this session.", supporting: support, alternatives: []string{"An intermittent issue may not have occurred during the measurement window.", "Public targets and ICMP handling may not represent every application path."}}}, evidence, true
	}
	if wifiRoute && wifiUnstable && gatewayStable {
		evidence = append(evidence, model.Evidence{ID: "contradictory-local-signals", Statement: "Wi-Fi instability indicators were present while gateway measurements remained stable.", Supports: false, SummaryKeys: append(keys(wifi), key(g))})
	}
	return nil, evidence, localStable
}

func metricMax(s model.CollectorSummary, name string) float64 {
	if stat, ok := s.Metrics[name]; ok {
		return stat.Max
	}
	return 0
}

func publicTargetGroups(byType map[string][]model.CollectorSummary, thresholds config.Thresholds) (degraded, healthy []model.CollectorSummary) {
	type targetState struct {
		degraded, healthy bool
		sample            model.CollectorSummary
	}
	states := map[string]targetState{}
	record := func(s model.CollectorSummary, isDegraded, isHealthy bool) {
		key := independentTargetKey(s.Target)
		item := states[key]
		item.degraded, item.healthy, item.sample = item.degraded || isDegraded, item.healthy || isHealthy, s
		states[key] = item
	}
	for _, s := range byType["tcp_connect"] {
		if !metricAdequate(s, "connect_ms", thresholds.MinimumSamples) {
			continue
		}
		record(s, s.FailureRate > .10 || s.Metrics["connect_ms"].Median >= thresholds.ConnectSlowMS, s.FailureRate <= .10 && s.Metrics["connect_ms"].Median <= thresholds.ConnectHealthyMS)
	}
	for _, s := range byType["https"] {
		if !metricAdequate(s, "total_ms", thresholds.MinimumSamples) {
			continue
		}
		record(s, s.FailureRate > .10 || s.Metrics["total_ms"].Median >= thresholds.HTTPSTotalSlowMS, s.FailureRate <= .10 && s.Metrics["total_ms"].Median < thresholds.HTTPSTotalSlowMS && (s.EndpointAttempts == 0 || s.EndpointSuccessRate >= .9))
	}
	for _, item := range states {
		if item.degraded && !item.healthy {
			degraded = append(degraded, item.sample)
		} else if item.healthy && !item.degraded {
			healthy = append(healthy, item.sample)
		}
	}
	return degraded, healthy
}

func independentTargetKey(target string) string {
	lower := strings.ToLower(target)
	if strings.Contains(lower, "cloudflare") || strings.Contains(lower, "1.1.1.1") {
		return "cloudflare"
	}
	if strings.Contains(lower, "google") || strings.Contains(lower, "8.8.8.8") {
		return "google"
	}
	return lower
}

func (e Engine) inconclusivePatternEvidence(byType map[string][]model.CollectorSummary) []model.Evidence {
	evidence := []model.Evidence{}
	system := byType["dns_system"]
	if len(system) == 1 && metricAdequate(system[0], "dns_ms", e.thresholds.MinimumSamples) && system[0].Metrics["dns_ms"].Median >= e.thresholds.DNSSlowMS {
		healthyComparison := false
		for _, summary := range byType["dns_comparison"] {
			if adequate(summary, "dns_ms", e.thresholds.MinimumSamples) && summary.Metrics["dns_ms"].Median <= e.thresholds.DNSHealthyMS {
				healthyComparison = true
			}
		}
		if !healthyComparison {
			evidence = append(evidence, model.Evidence{
				ID:          "slow-dns-without-healthy-contrast",
				Statement:   fmt.Sprintf("System DNS median was %.1f ms, above the %.1f ms threshold, but no comparison resolver met the healthy contrast requirement; DNS degradation could not be isolated.", system[0].Metrics["dns_ms"].Median, e.thresholds.DNSSlowMS),
				Supports:    true,
				SummaryKeys: append(keys(system), keys(byType["dns_comparison"])...),
			})
		}
	}
	for _, summary := range byType["dns_comparison"] {
		if summary.FailureRate > .10 || summary.LongestFailureRun > 1 {
			evidence = append(evidence, model.Evidence{
				ID:          "repeated-comparison-dns-failures",
				Statement:   fmt.Sprintf("%s had a %.1f%% failure rate and a longest run of %d consecutive failures.", key(summary), 100*summary.FailureRate, summary.LongestFailureRun),
				Supports:    true,
				SummaryKeys: []string{key(summary)},
			})
		}
	}
	return evidence
}

func (e Engine) deviceCandidate(s []model.CollectorSummary) (candidate, bool) {
	if len(s) != 1 || !adequate(s[0], "load_per_core", e.thresholds.MinimumSamples) || !adequate(s[0], "memory_available_percent", e.thresholds.MinimumSamples) {
		return candidate{}, false
	}
	load, memory := s[0].Metrics["load_per_core"].Median, s[0].Metrics["memory_available_percent"].Median
	if load < e.thresholds.CPULoadPerCoreHigh && memory >= e.thresholds.MemoryAvailableLowPC {
		return candidate{}, false
	}
	ev := model.Evidence{ID: "device-pressure", Statement: fmt.Sprintf("Device median load per core was %.2f and available memory was %.1f%%.", load, memory), Supports: true, SummaryKeys: []string{key(s[0])}}
	return candidate{code: model.DiagnosisDevice, conclusion: "Local device resource pressure is a likely contributor.", supporting: []model.Evidence{ev}, alternatives: []string{"A resource-intensive task may be temporary and unrelated to the network.", "Another unmeasured network issue may coexist."}}, true
}

func (e Engine) dnsCandidate(system, comparisons []model.CollectorSummary) (candidate, bool) {
	if len(system) != 1 || !adequate(system[0], "dns_ms", e.thresholds.MinimumSamples) || system[0].Metrics["dns_ms"].Median < e.thresholds.DNSSlowMS {
		return candidate{}, false
	}
	healthy := []model.CollectorSummary{}
	for _, s := range comparisons {
		if adequate(s, "dns_ms", e.thresholds.MinimumSamples) && s.Metrics["dns_ms"].Median <= e.thresholds.DNSHealthyMS {
			healthy = append(healthy, s)
		}
	}
	if len(healthy) == 0 {
		return candidate{}, false
	}
	support := []model.Evidence{{ID: "slow-system-dns", Statement: fmt.Sprintf("System DNS median was %.1f ms, above the %.1f ms threshold.", system[0].Metrics["dns_ms"].Median, e.thresholds.DNSSlowMS), Supports: true, SummaryKeys: []string{key(system[0])}}, {ID: "healthy-comparison-dns", Statement: "At least one comparison resolver remained within the healthy DNS threshold.", Supports: true, SummaryKeys: keys(healthy)}}
	return candidate{code: model.DiagnosisDNS, conclusion: "The system DNS path is likely degraded relative to comparison resolvers.", supporting: support, alternatives: []string{"Resolver caching or transient hostname-specific behavior may affect the comparison.", "The configured comparison resolver may use a different network path."}}, true
}

func (e Engine) destinationCandidate(s []model.CollectorSummary) (candidate, bool) {
	slow, healthy := []model.CollectorSummary{}, []model.CollectorSummary{}
	for _, summary := range s {
		if !adequate(summary, "connect_ms", e.thresholds.MinimumSamples) {
			continue
		}
		median := summary.Metrics["connect_ms"].Median
		if median >= e.thresholds.ConnectSlowMS {
			slow = append(slow, summary)
		} else if median <= e.thresholds.ConnectHealthyMS {
			healthy = append(healthy, summary)
		}
	}
	if len(slow) != 1 || len(healthy) == 0 {
		return candidate{}, false
	}
	support := []model.Evidence{{ID: "one-slow-destination", Statement: fmt.Sprintf("TCP connection median to %s was %.1f ms, above the %.1f ms threshold.", slow[0].Target, slow[0].Metrics["connect_ms"].Median, e.thresholds.ConnectSlowMS), Supports: true, SummaryKeys: []string{key(slow[0])}}, {ID: "healthy-alternative-destination", Statement: "At least one alternative TCP destination remained within the healthy threshold.", Supports: true, SummaryKeys: keys(healthy)}}
	return candidate{code: model.DiagnosisDestination, conclusion: "Degradation appears specific to one measured destination.", supporting: support, alternatives: []string{"Routing to that destination may be transiently different.", "The destination may intentionally delay or rate-limit connections."}}, true
}

func (e Engine) healthyAssessment(byType map[string][]model.CollectorSummary) (bool, []model.Evidence) {
	issues := []model.Evidence{}
	if len(byType["dns_system"]) != 1 || !metricAdequate(byType["dns_system"][0], "dns_ms", e.thresholds.MinimumSamples) || byType["dns_system"][0].Metrics["dns_ms"].Median >= e.thresholds.DNSSlowMS {
		return false, issues
	}
	if system := byType["dns_system"][0]; system.FailureRate > .10 || system.LongestFailureRun > 1 {
		issues = append(issues, model.Evidence{ID: "repeated-probe-failures", Statement: fmt.Sprintf("%s had a %.1f%% failure rate and a longest run of %d consecutive failures.", key(system), 100*system.FailureRate, system.LongestFailureRun), Supports: true, SummaryKeys: []string{key(system)}})
	}
	if len(byType["dns_comparison"]) == 0 || len(byType["tcp_connect"]) < 2 {
		return false, issues
	}
	comparisonAvailable := false
	for _, summary := range byType["dns_comparison"] {
		if adequate(summary, "dns_ms", e.thresholds.MinimumSamples) {
			comparisonAvailable = true
		}
		if summary.FailureRate > .10 || summary.LongestFailureRun > 1 {
			issues = append(issues, model.Evidence{ID: "repeated-comparison-dns-failures", Statement: fmt.Sprintf("%s had a %.1f%% failure rate and a longest run of %d consecutive failures.", key(summary), 100*summary.FailureRate, summary.LongestFailureRun), Supports: true, SummaryKeys: []string{key(summary)}})
		}
	}
	if !comparisonAvailable {
		return false, issues
	}
	checks := []struct {
		summaries []model.CollectorSummary
		metric    string
		threshold float64
	}{
		{byType["dns_system"], "dns_ms", e.thresholds.DNSSlowMS},
		{byType["tcp_connect"], "connect_ms", e.thresholds.ConnectSlowMS},
		{byType["https"], "total_ms", e.thresholds.HTTPSTotalSlowMS},
	}
	for _, check := range checks {
		for _, s := range check.summaries {
			if !below(s, check.metric, e.thresholds.MinimumSamples, check.threshold) {
				return false, issues
			}
			if s.FailureRate > .10 || s.LongestFailureRun > 1 {
				issues = append(issues, model.Evidence{ID: "repeated-probe-failures", Statement: fmt.Sprintf("%s had a %.1f%% failure rate and a longest run of %d consecutive failures.", key(s), 100*s.FailureRate, s.LongestFailureRun), Supports: true, SummaryKeys: []string{key(s)}})
			}
			if stat := s.Metrics[check.metric]; stat.P95 != nil && *stat.P95 >= check.threshold {
				issues = append(issues, model.Evidence{ID: "severe-tail-latency", Statement: fmt.Sprintf("%s p95 %s was %.1f ms, at or above the %.1f ms threshold, while its median was %.1f ms.", key(s), check.metric, *stat.P95, check.threshold, stat.Median), Supports: true, SummaryKeys: []string{key(s)}})
			}
			if s.CollectorType == "https" && s.EndpointAttempts > 0 && (s.EndpointSuccesses < e.thresholds.MinimumSamples || s.EndpointSuccessRate < .90) {
				status := s.Metrics["status_code"].Median
				issues = append(issues, model.Evidence{ID: "unexpected-endpoint-response", Statement: fmt.Sprintf("HTTPS transport to %s completed, but only %d of %d responses matched the configured expected status codes (observed median status %.0f).", s.Target, s.EndpointSuccesses, s.EndpointAttempts, status), Supports: true, SummaryKeys: []string{key(s)}})
			}
		}
	}
	if len(byType["https"]) == 0 {
		return false, issues
	}
	if len(byType["device_resources"]) != 1 || !adequate(byType["device_resources"][0], "load_per_core", e.thresholds.MinimumSamples) || byType["device_resources"][0].Metrics["load_per_core"].Median >= e.thresholds.CPULoadPerCoreHigh || byType["device_resources"][0].Metrics["memory_available_percent"].Median < e.thresholds.MemoryAvailableLowPC {
		return false, issues
	}
	return len(issues) == 0, issues
}

func instabilityEvidence(summaries []model.CollectorSummary) *model.Evidence {
	timeouts, outliers := 0, 0
	unstable := []model.CollectorSummary{}
	for _, summary := range summaries {
		summaryOutliers := 0
		for name, stat := range summary.Metrics {
			if name == "dns_ms" || name == "connect_ms" || name == "total_ms" || name == "ttfb_ms" {
				summaryOutliers += stat.OutlierCount
			}
		}
		if summary.Errors[model.ErrorTimeout] > 0 || summaryOutliers > 0 {
			unstable = append(unstable, summary)
			timeouts += summary.Errors[model.ErrorTimeout]
			outliers += summaryOutliers
		}
	}
	if len(unstable) == 0 {
		return nil
	}
	return &model.Evidence{ID: "isolated-instability", Statement: fmt.Sprintf("Typical measurements remained below thresholds, while %d isolated timeout(s) and %d robust latency outlier(s) were observed.", timeouts, outliers), Supports: true, SummaryKeys: keys(unstable)}
}

func adequate(s model.CollectorSummary, metric string, minimum int) bool {
	return metricAdequate(s, metric, minimum) && s.FailureRate <= .10 && s.LongestFailureRun <= 1
}
func metricAdequate(s model.CollectorSummary, metric string, minimum int) bool {
	stat, ok := s.Metrics[metric]
	return ok && s.Successes >= minimum && stat.Count >= minimum
}
func below(s model.CollectorSummary, metric string, minimum int, threshold float64) bool {
	return adequate(s, metric, minimum) && s.Metrics[metric].Median < threshold
}
func key(s model.CollectorSummary) string { return s.CollectorType + ":" + s.Target }
func keys(s []model.CollectorSummary) []string {
	out := make([]string, len(s))
	for i := range s {
		out[i] = key(s[i])
	}
	sort.Strings(out)
	return out
}
func unknown(conclusion string, evidence []model.Evidence) model.Diagnosis {
	return model.Diagnosis{Code: model.DiagnosisUnknown, Conclusion: conclusion, SupportingEvidence: evidenceSlice(evidence), ContradictingEvidence: []model.Evidence{}, Alternatives: []string{"The issue may be intermittent.", "VPN routing, ICMP policy, unmeasured throughput, or another unsupported path may contribute."}}
}

func evidenceSlice(values []model.Evidence) []model.Evidence {
	if values == nil {
		return []model.Evidence{}
	}
	return values
}
func stringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
