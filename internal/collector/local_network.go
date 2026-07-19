package collector

import (
	"context"
	"errors"
	"sync"
	"time"

	"whyfi/internal/model"
	"whyfi/internal/platform"
)

type WiFiProbe struct {
	base
	api               platform.WiFiAPI
	mu                sync.Mutex
	havePrevious      bool
	previousConnected bool
	reconnects        int
}

func NewWiFi(api platform.WiFiAPI, clock Clock, id IDGenerator) Probe {
	return &WiFiProbe{base: base{kind: "wifi", target: "Wi-Fi network 1", clock: clock, id: id}, api: api}
}

func (p *WiFiProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	started := p.clock.Now()
	if err := ctx.Err(); err != nil {
		return p.finish(r, started, err)
	}
	snapshot, err := p.api.Snapshot(ctx)
	if err != nil {
		return p.finishWithContext(ctx, r, started, err)
	}
	p.mu.Lock()
	stateChange := p.havePrevious && snapshot.Connected != p.previousConnected
	if stateChange && snapshot.Connected {
		p.reconnects++
	}
	p.previousConnected, p.havePrevious = snapshot.Connected, true
	reconnects := p.reconnects
	p.mu.Unlock()
	connected, changed := 0.0, 0.0
	if snapshot.Connected {
		connected = 1
	}
	if stateChange {
		changed = 1
	}
	r.Values = map[string]float64{"connected": connected, "state_change": changed, "reconnect_count": float64(reconnects)}
	if snapshot.Connected {
		r.Values["signal_quality_percent"] = float64(snapshot.SignalQuality)
		r.Values["receive_link_kbps"] = float64(snapshot.ReceiveKbps)
		r.Values["transmit_link_kbps"] = float64(snapshot.TransmitKbps)
	}
	r.Details = map[string]string{"network_label": "Wi-Fi network 1", "adapter_label": "Wi-Fi adapter 1", "radio": snapshot.Radio}
	// snapshot.SSID is intentionally never copied into the persisted result.
	return p.finishWithContext(ctx, r, started, nil)
}

type GatewayProbe struct {
	base
	route   platform.RouteAPI
	echo    platform.EchoAPI
	timeout time.Duration
}

func NewGateway(route platform.RouteAPI, echo platform.EchoAPI, timeout time.Duration, clock Clock, id IDGenerator) Probe {
	return &GatewayProbe{base: base{kind: "gateway", target: "local_gateway", clock: clock, id: id}, route: route, echo: echo, timeout: timeout}
}

func (p *GatewayProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	started := p.clock.Now()
	if err := ctx.Err(); err != nil {
		return p.finish(r, started, err)
	}
	route, err := p.route.DefaultIPv4Route(ctx)
	if err != nil {
		return p.finishWithContext(ctx, r, started, err)
	}
	r.Values = map[string]float64{"default_route_present": 1}
	r.Details = map[string]string{"connection_kind": string(route.Kind), "gateway_label": "local gateway 1"}
	reply, err := p.echo.Echo(ctx, route.Gateway, p.timeout)
	if err != nil {
		if errors.Is(err, platform.ErrICMPUnavailable) {
			r.Details["reachability_method"] = "route_only"
		}
		return p.finishWithContext(ctx, r, started, err)
	}
	r.Values["gateway_ms"] = float64(reply.Latency.Microseconds()) / 1000
	r.Details["reachability_method"] = reply.Method
	return p.finishWithContext(ctx, r, started, nil)
}
