package session

import (
	"math"
	"sort"

	"whyfi/internal/model"
)

func Summarize(results []model.ProbeResult) []model.CollectorSummary {
	type group struct{ results []model.ProbeResult }
	groups := map[string]*group{}
	for _, result := range results {
		key := result.CollectorType + "\x00" + result.Target
		if groups[key] == nil {
			groups[key] = &group{}
		}
		groups[key].results = append(groups[key].results, result)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]model.CollectorSummary, 0, len(keys))
	for _, key := range keys {
		g := groups[key]
		s := model.CollectorSummary{CollectorType: g.results[0].CollectorType, Target: g.results[0].Target, Attempts: len(g.results), Errors: map[model.ErrorCategory]int{}, Metrics: map[string]model.Statistics{}, DetailCounts: map[string]int{}}
		values := map[string][]float64{}
		for _, r := range g.results {
			for name, value := range r.Details {
				s.DetailCounts[name+"="+value]++
			}
			if r.TransportSuccess != nil && *r.TransportSuccess {
				s.TransportSuccesses++
			}
			if r.EndpointSuccess != nil {
				s.EndpointAttempts++
				if *r.EndpointSuccess {
					s.EndpointSuccesses++
				}
			}
			if r.CollectorType == "gateway" && !r.Success && r.ErrorCategory != model.ErrorICMPBlocked && r.Values["default_route_present"] == 1 {
				s.Losses++
			}
			if r.Success {
				s.Successes++
				for name, value := range r.Values {
					values[name] = append(values[name], value)
				}
			} else {
				s.Errors[r.ErrorCategory]++
			}
		}
		s.SuccessRate = float64(s.Successes) / float64(s.Attempts)
		s.FailureRate = 1 - s.SuccessRate
		s.TimeoutRate = float64(s.Errors[model.ErrorTimeout]) / float64(s.Attempts)
		s.LongestFailureRun = longestFailureRun(g.results)
		if s.CollectorType == "https" {
			s.TransportSuccessRate = float64(s.TransportSuccesses) / float64(s.Attempts)
			if s.EndpointAttempts > 0 {
				s.EndpointSuccessRate = float64(s.EndpointSuccesses) / float64(s.EndpointAttempts)
			}
		}
		if s.CollectorType == "gateway" {
			s.LossRate = float64(s.Losses) / float64(s.Attempts)
		}
		for name, samples := range values {
			s.Metrics[name] = statistics(samples)
		}
		if len(s.Errors) == 0 {
			s.Errors = nil
		}
		if len(s.Metrics) == 0 {
			s.Metrics = nil
		}
		if len(s.DetailCounts) == 0 {
			s.DetailCounts = nil
		}
		out = append(out, s)
	}
	return out
}

func statistics(values []float64) model.Statistics {
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	total := 0.0
	for _, value := range values {
		total += value
	}
	median := quantile(values, .5)
	deviations := make([]float64, len(values))
	for i, value := range values {
		deviations[i] = math.Abs(value - median)
	}
	sort.Float64s(deviations)
	mad := quantile(deviations, .5)
	outliers := 0
	if mad > 0 {
		limit := 3 * 1.4826 * mad
		for _, value := range values {
			if math.Abs(value-median) > limit {
				outliers++
			}
		}
	}
	stat := model.Statistics{Count: len(values), Min: values[0], Median: median, Max: values[len(values)-1], Mean: total / float64(len(values)), MAD: mad, OutlierCount: outliers}
	if len(values) >= 20 {
		value := nearestRank(values, .95)
		stat.P95 = &value
	}
	if len(values) >= 100 {
		value := nearestRank(values, .99)
		stat.P99 = &value
	}
	return stat
}

func quantile(sorted []float64, p float64) float64 {
	position := p * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	return sorted[lower] + (sorted[upper]-sorted[lower])*(position-float64(lower))
}

func nearestRank(sorted []float64, p float64) float64 {
	return sorted[int(math.Ceil(p*float64(len(sorted))))-1]
}

func longestFailureRun(results []model.ProbeResult) int {
	ordered := append([]model.ProbeResult(nil), results...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].StartedAt.Before(ordered[j].StartedAt) })
	longest, current := 0, 0
	for _, result := range ordered {
		if result.Success {
			current = 0
			continue
		}
		current++
		if current > longest {
			longest = current
		}
	}
	return longest
}
