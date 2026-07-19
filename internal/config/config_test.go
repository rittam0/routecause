package config

import (
	"testing"
	"time"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{"zero duration", func(c *Config) { c.Duration = 0 }},
		{"zero DNS interval", func(c *Config) { c.DNSInterval = 0 }},
		{"zero Wi-Fi interval", func(c *Config) { c.WiFiInterval = 0 }},
		{"zero gateway interval", func(c *Config) { c.GatewayInterval = 0 }},
		{"one TCP target", func(c *Config) { c.TCPDestinations = []string{"example.com:443"} }},
		{"bad resolver", func(c *Config) { c.ComparisonResolvers = []string{"1.1.1.1"} }},
		{"bad URL", func(c *Config) { c.HTTPSHealthEndpoint = "http://example.com" }},
		{"bad method", func(c *Config) { c.HTTPSSecondaryMethod = "POST" }},
		{"missing expected status", func(c *Config) { c.HTTPSHealthStatuses = nil }},
		{"bad expected status", func(c *Config) { c.HTTPSHealthStatuses = []int{700} }},
		{"bad Wi-Fi threshold", func(c *Config) { c.Thresholds.WiFiWeakPercent = 100 }},
		{"inverted DNS thresholds", func(c *Config) { c.Thresholds.DNSSlowMS = c.Thresholds.DNSHealthyMS }},
		{"short timeout is allowed", func(c *Config) { c.ProbeTimeout = time.Nanosecond }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := Default()
			test.change(&c)
			err := c.Validate()
			if test.name == "short timeout is allowed" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
