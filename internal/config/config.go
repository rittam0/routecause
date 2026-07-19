package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"whyfi/internal/model"
)

type Thresholds struct {
	MinimumSamples       int
	DNSSlowMS            float64
	DNSHealthyMS         float64
	ConnectSlowMS        float64
	ConnectHealthyMS     float64
	HTTPSTotalSlowMS     float64
	CPULoadPerCoreHigh   float64
	MemoryAvailableLowPC float64
	WiFiWeakPercent      float64
	GatewaySlowMS        float64
}

type Config struct {
	Duration               time.Duration
	DeviceInterval         time.Duration
	DNSInterval            time.Duration
	TCPInterval            time.Duration
	HTTPSInterval          time.Duration
	WiFiInterval           time.Duration
	GatewayInterval        time.Duration
	ProbeTimeout           time.Duration
	MaxConcurrency         int
	DNSHostname            string
	ComparisonResolvers    []string
	TCPDestinations        []string
	HTTPSHealthEndpoint    string
	HTTPSSecondaryEndpoint string
	HTTPSHealthMethod      string
	HTTPSHealthStatuses    []int
	HTTPSSecondaryMethod   string
	HTTPSSecondaryStatuses []int
	Thresholds             Thresholds
}

func Default() Config {
	return Config{
		Duration: 30 * time.Second, DeviceInterval: time.Second, DNSInterval: 2 * time.Second, TCPInterval: 2 * time.Second, HTTPSInterval: 10 * time.Second, WiFiInterval: time.Second, GatewayInterval: 2 * time.Second, ProbeTimeout: 3 * time.Second, MaxConcurrency: 4,
		DNSHostname: "example.com", ComparisonResolvers: []string{"1.1.1.1:53", "8.8.8.8:53"},
		TCPDestinations:        []string{"1.1.1.1:443", "8.8.8.8:443"},
		HTTPSHealthEndpoint:    "https://www.google.com/generate_204",
		HTTPSSecondaryEndpoint: "https://www.cloudflare.com/cdn-cgi/trace",
		HTTPSHealthMethod:      "HEAD", HTTPSHealthStatuses: []int{204},
		HTTPSSecondaryMethod: "GET", HTTPSSecondaryStatuses: []int{200},
		Thresholds: Thresholds{MinimumSamples: 2, DNSSlowMS: 150, DNSHealthyMS: 80, ConnectSlowMS: 250, ConnectHealthyMS: 120, HTTPSTotalSlowMS: 1500, CPULoadPerCoreHigh: 1.0, MemoryAvailableLowPC: 10, WiFiWeakPercent: 40, GatewaySlowMS: 50},
	}
}

func (c Config) Validate() error {
	if c.Duration <= 0 || c.DeviceInterval <= 0 || c.DNSInterval <= 0 || c.TCPInterval <= 0 || c.HTTPSInterval <= 0 || c.WiFiInterval <= 0 || c.GatewayInterval <= 0 || c.ProbeTimeout <= 0 {
		return errors.New("duration, collector intervals, and probe timeout must be positive")
	}
	if c.MaxConcurrency < 1 {
		return errors.New("max concurrency must be at least 1")
	}
	if strings.TrimSpace(c.DNSHostname) == "" {
		return errors.New("DNS hostname is required")
	}
	if c.Thresholds.MinimumSamples < 1 {
		return errors.New("minimum samples must be at least 1")
	}
	if c.Thresholds.DNSHealthyMS < 0 || c.Thresholds.DNSSlowMS <= c.Thresholds.DNSHealthyMS || c.Thresholds.ConnectHealthyMS < 0 || c.Thresholds.ConnectSlowMS <= c.Thresholds.ConnectHealthyMS || c.Thresholds.HTTPSTotalSlowMS <= 0 || c.Thresholds.CPULoadPerCoreHigh <= 0 || c.Thresholds.MemoryAvailableLowPC <= 0 || c.Thresholds.MemoryAvailableLowPC >= 100 {
		return errors.New("diagnosis thresholds must be positive and healthy thresholds must be below slow thresholds")
	}
	if c.Thresholds.WiFiWeakPercent <= 0 || c.Thresholds.WiFiWeakPercent >= 100 || c.Thresholds.GatewaySlowMS <= 0 {
		return errors.New("Wi-Fi and gateway thresholds must be positive and valid")
	}
	for _, resolver := range c.ComparisonResolvers {
		if _, _, err := net.SplitHostPort(resolver); err != nil {
			return fmt.Errorf("invalid resolver %q: %w", resolver, err)
		}
	}
	for _, destination := range c.TCPDestinations {
		if _, _, err := net.SplitHostPort(destination); err != nil {
			return fmt.Errorf("invalid TCP destination %q: %w", destination, err)
		}
	}
	for role, destination := range map[string]string{"health": c.HTTPSHealthEndpoint, "secondary": c.HTTPSSecondaryEndpoint} {
		u, err := url.Parse(destination)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("invalid HTTPS %s endpoint %q", role, destination)
		}
	}
	for role, endpoint := range map[string]struct {
		method   string
		statuses []int
	}{
		"health": {c.HTTPSHealthMethod, c.HTTPSHealthStatuses}, "secondary": {c.HTTPSSecondaryMethod, c.HTTPSSecondaryStatuses},
	} {
		method := strings.ToUpper(strings.TrimSpace(endpoint.method))
		if method != "HEAD" && method != "GET" {
			return fmt.Errorf("HTTPS %s method must be HEAD or GET", role)
		}
		if len(endpoint.statuses) == 0 {
			return fmt.Errorf("HTTPS %s expected statuses are required", role)
		}
		for _, status := range endpoint.statuses {
			if status < 100 || status > 599 {
				return fmt.Errorf("invalid HTTPS %s expected status %d", role, status)
			}
		}
	}
	if len(c.TCPDestinations) < 2 {
		return errors.New("at least two TCP destinations are required for comparison")
	}
	return nil
}

func (c Config) Snapshot() model.SessionConfiguration {
	t := c.Thresholds
	return model.SessionConfiguration{Duration: c.Duration.String(), DeviceInterval: c.DeviceInterval.String(), DNSInterval: c.DNSInterval.String(), TCPInterval: c.TCPInterval.String(), HTTPSInterval: c.HTTPSInterval.String(), WiFiInterval: c.WiFiInterval.String(), GatewayInterval: c.GatewayInterval.String(), ProbeTimeout: c.ProbeTimeout.String(), DNSHostname: c.DNSHostname, ComparisonResolvers: c.ComparisonResolvers, TCPDestinations: c.TCPDestinations, HTTPSHealthEndpoint: c.HTTPSHealthEndpoint, HTTPSSecondaryEndpoint: c.HTTPSSecondaryEndpoint, HTTPSHealthMethod: strings.ToUpper(c.HTTPSHealthMethod), HTTPSHealthStatuses: c.HTTPSHealthStatuses, HTTPSSecondaryMethod: strings.ToUpper(c.HTTPSSecondaryMethod), HTTPSSecondaryStatuses: c.HTTPSSecondaryStatuses, Thresholds: model.ThresholdConfiguration{MinimumSamples: t.MinimumSamples, DNSSlowMS: t.DNSSlowMS, DNSHealthyMS: t.DNSHealthyMS, ConnectSlowMS: t.ConnectSlowMS, ConnectHealthyMS: t.ConnectHealthyMS, HTTPSTotalSlowMS: t.HTTPSTotalSlowMS, CPULoadPerCoreHigh: t.CPULoadPerCoreHigh, MemoryAvailableLowPC: t.MemoryAvailableLowPC, WiFiWeakPercent: t.WiFiWeakPercent, GatewaySlowMS: t.GatewaySlowMS}}
}
