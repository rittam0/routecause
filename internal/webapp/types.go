package webapp

import (
	"context"
	"time"

	"whyfi/internal/model"
)

type Runner interface {
	Run(context.Context, time.Duration, func(model.ProbeResult)) (model.DiagnosticSession, error)
}

type LayerState struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Observation string `json:"observation,omitempty"`
}

type Event struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Data any    `json:"data"`
}

type HistoryItem struct {
	ID         string              `json:"id"`
	StartedAt  time.Time           `json:"started_at"`
	EndedAt    *time.Time          `json:"ended_at,omitempty"`
	Duration   string              `json:"duration"`
	Status     string              `json:"status"`
	Code       model.DiagnosisCode `json:"diagnosis_code,omitempty"`
	Conclusion string              `json:"conclusion,omitempty"`
	Confidence string              `json:"confidence,omitempty"`
	Demo       bool                `json:"demo"`
	Scenario   string              `json:"demo_scenario,omitempty"`
	Layers     []LayerState        `json:"layers"`
}

type StartRequest struct {
	Duration     string `json:"duration"`
	DemoScenario string `json:"demo_scenario,omitempty"`
}

type StartResponse struct {
	ID         string `json:"id"`
	EventsURL  string `json:"events_url"`
	SessionURL string `json:"session_url"`
}

var layerOrder = []LayerState{
	{ID: "local", Label: "Wi-Fi / Ethernet", Status: "waiting"},
	{ID: "gateway", Label: "Gateway", Status: "waiting"},
	{ID: "dns", Label: "DNS", Status: "waiting"},
	{ID: "internet", Label: "Internet", Status: "waiting"},
	{ID: "destinations", Label: "Tested destinations", Status: "waiting"},
}
