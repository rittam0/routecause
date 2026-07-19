package model

import "time"

const SchemaVersion = "1.3.0"

type ErrorCategory string

const (
	ErrorNone         ErrorCategory = ""
	ErrorTimeout      ErrorCategory = "timeout"
	ErrorRefused      ErrorCategory = "connection_refused"
	ErrorDNS          ErrorCategory = "dns_error"
	ErrorTLS          ErrorCategory = "tls_error"
	ErrorHTTP         ErrorCategory = "http_error"
	ErrorCancelled    ErrorCategory = "cancelled"
	ErrorUnsupported  ErrorCategory = "unsupported"
	ErrorSystem       ErrorCategory = "system_error"
	ErrorNoAdapter    ErrorCategory = "no_adapter"
	ErrorDisconnected ErrorCategory = "disconnected"
	ErrorAmbiguous    ErrorCategory = "ambiguous_route"
	ErrorICMPBlocked  ErrorCategory = "icmp_unavailable"
)

type ProbeResult struct {
	ID               string             `json:"id"`
	CollectorType    string             `json:"collector_type"`
	Target           string             `json:"target"`
	StartedAt        time.Time          `json:"started_at"`
	DurationMS       float64            `json:"duration_ms"`
	Success          bool               `json:"success"`
	TransportSuccess *bool              `json:"transport_success,omitempty"`
	EndpointSuccess  *bool              `json:"endpoint_success,omitempty"`
	HTTPMethod       string             `json:"http_method,omitempty"`
	Details          map[string]string  `json:"details,omitempty"`
	ErrorCategory    ErrorCategory      `json:"error_category,omitempty"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	Values           map[string]float64 `json:"values,omitempty"`
	OSContext        map[string]string  `json:"os_context,omitempty"`
}

type Statistics struct {
	Count        int      `json:"count"`
	Min          float64  `json:"min"`
	Median       float64  `json:"median"`
	P95          *float64 `json:"p95,omitempty"`
	P99          *float64 `json:"p99,omitempty"`
	Max          float64  `json:"max"`
	Mean         float64  `json:"mean"`
	MAD          float64  `json:"median_absolute_deviation"`
	OutlierCount int      `json:"outlier_count"`
}

type CollectorSummary struct {
	CollectorType        string                `json:"collector_type"`
	Target               string                `json:"target"`
	Attempts             int                   `json:"attempts"`
	Successes            int                   `json:"successes"`
	SuccessRate          float64               `json:"success_rate"`
	FailureRate          float64               `json:"failure_rate"`
	TimeoutRate          float64               `json:"timeout_rate"`
	LongestFailureRun    int                   `json:"longest_consecutive_failures"`
	TransportSuccesses   int                   `json:"transport_successes,omitempty"`
	TransportSuccessRate float64               `json:"transport_success_rate,omitempty"`
	EndpointAttempts     int                   `json:"endpoint_attempts,omitempty"`
	EndpointSuccesses    int                   `json:"endpoint_successes,omitempty"`
	EndpointSuccessRate  float64               `json:"endpoint_success_rate,omitempty"`
	Losses               int                   `json:"losses,omitempty"`
	LossRate             float64               `json:"loss_rate,omitempty"`
	Errors               map[ErrorCategory]int `json:"errors,omitempty"`
	Metrics              map[string]Statistics `json:"metrics,omitempty"`
	DetailCounts         map[string]int        `json:"detail_counts,omitempty"`
}

type Evidence struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement"`
	Supports    bool     `json:"supports"`
	SummaryKeys []string `json:"summary_keys"`
}

type DiagnosisCode string

const (
	DiagnosisHealthy     DiagnosisCode = "no_material_problem_observed"
	DiagnosisDNS         DiagnosisCode = "likely_dns_degradation"
	DiagnosisDestination DiagnosisCode = "likely_destination_specific_degradation"
	DiagnosisDevice      DiagnosisCode = "device_resource_pressure"
	DiagnosisLocalWiFi   DiagnosisCode = "likely_local_wifi_instability"
	DiagnosisGateway     DiagnosisCode = "likely_local_gateway_instability"
	DiagnosisUpstream    DiagnosisCode = "likely_upstream_degradation"
	DiagnosisUnknown     DiagnosisCode = "unknown"
)

type Diagnosis struct {
	Code                  DiagnosisCode `json:"code"`
	Conclusion            string        `json:"conclusion"`
	Confidence            string        `json:"confidence,omitempty"`
	SupportingEvidence    []Evidence    `json:"supporting_evidence"`
	ContradictingEvidence []Evidence    `json:"contradicting_evidence"`
	Alternatives          []string      `json:"alternative_explanations"`
}

type SessionConfiguration struct {
	Duration               string                 `json:"duration"`
	DeviceInterval         string                 `json:"device_interval"`
	DNSInterval            string                 `json:"dns_interval"`
	TCPInterval            string                 `json:"tcp_interval"`
	HTTPSInterval          string                 `json:"https_interval"`
	WiFiInterval           string                 `json:"wifi_interval"`
	GatewayInterval        string                 `json:"gateway_interval"`
	ProbeTimeout           string                 `json:"probe_timeout"`
	DNSHostname            string                 `json:"dns_hostname"`
	ComparisonResolvers    []string               `json:"comparison_resolvers"`
	TCPDestinations        []string               `json:"tcp_destinations"`
	HTTPSHealthEndpoint    string                 `json:"https_health_endpoint"`
	HTTPSSecondaryEndpoint string                 `json:"https_secondary_endpoint"`
	HTTPSHealthMethod      string                 `json:"https_health_method"`
	HTTPSHealthStatuses    []int                  `json:"https_health_expected_statuses"`
	HTTPSSecondaryMethod   string                 `json:"https_secondary_method"`
	HTTPSSecondaryStatuses []int                  `json:"https_secondary_expected_statuses"`
	Thresholds             ThresholdConfiguration `json:"thresholds"`
}

type ThresholdConfiguration struct {
	MinimumSamples       int     `json:"minimum_samples"`
	DNSSlowMS            float64 `json:"dns_slow_ms"`
	DNSHealthyMS         float64 `json:"dns_healthy_ms"`
	ConnectSlowMS        float64 `json:"connect_slow_ms"`
	ConnectHealthyMS     float64 `json:"connect_healthy_ms"`
	HTTPSTotalSlowMS     float64 `json:"https_total_slow_ms"`
	CPULoadPerCoreHigh   float64 `json:"cpu_load_per_core_high"`
	MemoryAvailableLowPC float64 `json:"memory_available_low_percent"`
	WiFiWeakPercent      float64 `json:"wifi_weak_percent"`
	GatewaySlowMS        float64 `json:"gateway_slow_ms"`
}

type DiagnosticSession struct {
	SchemaVersion string               `json:"schema_version"`
	ID            string               `json:"id"`
	Configuration SessionConfiguration `json:"configuration"`
	StartedAt     time.Time            `json:"started_at"`
	EndedAt       time.Time            `json:"ended_at"`
	RawResults    []ProbeResult        `json:"raw_results"`
	Summaries     []CollectorSummary   `json:"summaries"`
	Evidence      []Evidence           `json:"evidence"`
	Diagnosis     Diagnosis            `json:"diagnosis"`
	Limitations   []string             `json:"limitations"`
}
