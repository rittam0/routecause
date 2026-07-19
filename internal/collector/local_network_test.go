package collector

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"whyfi/internal/model"
	"whyfi/internal/platform"
)

type fakeWiFiAPI struct {
	snapshots []platform.WiFiSnapshot
	err       error
	index     int
}

func (f *fakeWiFiAPI) Snapshot(ctx context.Context) (platform.WiFiSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return platform.WiFiSnapshot{}, err
	}
	if f.err != nil {
		return platform.WiFiSnapshot{}, f.err
	}
	item := f.snapshots[f.index]
	if f.index < len(f.snapshots)-1 {
		f.index++
	}
	return item, nil
}

type fakeRouteAPI struct {
	route platform.DefaultRoute
	err   error
}

func (f fakeRouteAPI) DefaultIPv4Route(ctx context.Context) (platform.DefaultRoute, error) {
	if err := ctx.Err(); err != nil {
		return platform.DefaultRoute{}, err
	}
	return f.route, f.err
}

type fakeEchoAPI struct {
	reply platform.EchoReply
	err   error
}

func (f fakeEchoAPI) Echo(ctx context.Context, _ net.IP, _ time.Duration) (platform.EchoReply, error) {
	if err := ctx.Err(); err != nil {
		return platform.EchoReply{}, err
	}
	return f.reply, f.err
}

func TestWiFiCollectorRedactsPrivateIdentifiersAndTracksReconnect(t *testing.T) {
	api := &fakeWiFiAPI{snapshots: []platform.WiFiSnapshot{
		{AdapterPresent: true, Connected: true, SSID: "private-home-ssid", SignalQuality: 82, ReceiveKbps: 866000, TransmitKbps: 433000, Radio: "802.11ac"},
		{AdapterPresent: true, Connected: false, SSID: "private-home-ssid"},
		{AdapterPresent: true, Connected: true, SSID: "private-home-ssid", SignalQuality: 78},
	}}
	probe := NewWiFi(api, RealClock{}, func() string { return "id" })
	results := []model.ProbeResult{probe.Collect(context.Background()), probe.Collect(context.Background()), probe.Collect(context.Background())}
	if results[1].Values["state_change"] != 1 || results[2].Values["reconnect_count"] != 1 {
		t.Fatalf("reconnect was not tracked: %#v", results)
	}
	data, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"private-home-ssid", "bssid", "guid", "mac"} {
		if strings.Contains(strings.ToLower(text), secret) {
			t.Fatalf("private identifier %q persisted: %s", secret, text)
		}
	}
	if !strings.Contains(text, "Wi-Fi network 1") {
		t.Fatal("session-local network label missing")
	}
}

func TestWiFiCollectorTypedErrorsAndDeadline(t *testing.T) {
	probe := NewWiFi(&fakeWiFiAPI{err: platform.ErrNoWiFiAdapter}, RealClock{}, func() string { return "id" })
	if got := probe.Collect(context.Background()); got.ErrorCategory != model.ErrorNoAdapter {
		t.Fatalf("got %#v", got)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	defer cancel()
	if got := probe.Collect(ctx); got.ErrorCategory != model.ErrorTimeout {
		t.Fatalf("got %#v", got)
	}
	probe = NewWiFi(&fakeWiFiAPI{err: errors.New("WLAN API failed")}, RealClock{}, func() string { return "id" })
	if got := probe.Collect(context.Background()); got.ErrorCategory != model.ErrorSystem {
		t.Fatalf("got %#v", got)
	}
}

func TestWiFiCollectorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	probe := NewWiFi(&fakeWiFiAPI{snapshots: []platform.WiFiSnapshot{{Connected: true}}}, RealClock{}, func() string { return "id" })
	if got := probe.Collect(ctx); got.ErrorCategory != model.ErrorCancelled {
		t.Fatalf("got %#v", got)
	}
}

func TestGatewayCollectorICMPAndRouteOnlyFallback(t *testing.T) {
	route := fakeRouteAPI{route: platform.DefaultRoute{Gateway: net.IPv4(192, 0, 2, 1), Kind: platform.ConnectionWiFi}}
	probe := NewGateway(route, fakeEchoAPI{reply: platform.EchoReply{Latency: 7 * time.Millisecond, Method: "windows_icmp"}}, time.Second, RealClock{}, func() string { return "id" })
	got := probe.Collect(context.Background())
	if !got.Success || got.Values["gateway_ms"] != 7 || got.Details["reachability_method"] != "windows_icmp" {
		t.Fatalf("got %#v", got)
	}
	probe = NewGateway(route, fakeEchoAPI{err: platform.ErrICMPUnavailable}, time.Second, RealClock{}, func() string { return "id" })
	got = probe.Collect(context.Background())
	if got.Success || got.ErrorCategory != model.ErrorICMPBlocked || got.Values["default_route_present"] != 1 || got.Details["reachability_method"] != "route_only" {
		t.Fatalf("got %#v", got)
	}
}

func TestGatewayCollectorRouteAmbiguityAndAPIError(t *testing.T) {
	probe := NewGateway(fakeRouteAPI{err: platform.ErrRouteAmbiguous}, fakeEchoAPI{}, time.Second, RealClock{}, func() string { return "id" })
	if got := probe.Collect(context.Background()); got.ErrorCategory != model.ErrorAmbiguous {
		t.Fatalf("got %#v", got)
	}
	probe = NewGateway(fakeRouteAPI{err: errors.New("IP Helper failed")}, fakeEchoAPI{}, time.Second, RealClock{}, func() string { return "id" })
	if got := probe.Collect(context.Background()); got.ErrorCategory != model.ErrorSystem {
		t.Fatalf("got %#v", got)
	}
}

func TestGatewayCollectorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	probe := NewGateway(fakeRouteAPI{}, fakeEchoAPI{}, time.Second, RealClock{}, func() string { return "id" })
	if got := probe.Collect(ctx); got.ErrorCategory != model.ErrorCancelled {
		t.Fatalf("got %#v", got)
	}
}
