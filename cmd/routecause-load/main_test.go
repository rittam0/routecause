package main

import "testing"

func TestParseStages(t *testing.T) {
	got, err := parseStages("10, 100,10000")
	if err != nil || len(got) != 3 || got[2] != 10000 {
		t.Fatalf("parseStages = %v, %v", got, err)
	}
	for _, invalid := range []string{"0", "10001", "no"} {
		if _, err := parseStages(invalid); err == nil {
			t.Fatalf("parseStages(%q) succeeded", invalid)
		}
	}
}

func TestPercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	if got := percentile(values, .95); got != 5 {
		t.Fatalf("p95 = %v", got)
	}
}
