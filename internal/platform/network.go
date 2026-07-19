package platform

import (
	"context"
	"errors"
	"net"
	"time"
)

var (
	ErrNoWiFiAdapter   = errors.New("no native Wi-Fi adapter is available")
	ErrNotConnected    = errors.New("Wi-Fi adapter is not connected")
	ErrNoDefaultRoute  = errors.New("no IPv4 default route is available")
	ErrRouteAmbiguous  = errors.New("multiple equally preferred default routes are active")
	ErrICMPUnavailable = errors.New("Windows ICMP echo is unavailable or blocked")
)

type ConnectionKind string

const (
	ConnectionUnknown  ConnectionKind = "unknown"
	ConnectionWiFi     ConnectionKind = "wifi"
	ConnectionEthernet ConnectionKind = "ethernet"
	ConnectionVPN      ConnectionKind = "vpn_or_tunnel"
)

type WiFiSnapshot struct {
	AdapterPresent bool
	Connected      bool
	SSID           string // Sensitive: collectors must never persist this value.
	SignalQuality  uint32
	ReceiveKbps    uint32
	TransmitKbps   uint32
	Radio          string
}

type DefaultRoute struct {
	Gateway        net.IP
	InterfaceIndex uint32
	Kind           ConnectionKind
	Metric         uint32
	Ambiguous      bool
}

type EchoReply struct {
	Latency time.Duration
	Method  string
}

type WiFiAPI interface {
	Snapshot(context.Context) (WiFiSnapshot, error)
}
type RouteAPI interface {
	DefaultIPv4Route(context.Context) (DefaultRoute, error)
}
type EchoAPI interface {
	Echo(context.Context, net.IP, time.Duration) (EchoReply, error)
}

type NetworkAPIs struct {
	WiFi          WiFiAPI
	Route         RouteAPI
	Echo          EchoAPI
	NativeWindows bool
}

func NewNetworkAPIs() NetworkAPIs { return newNetworkAPIs() }

type CapabilityReport struct {
	NativeWindows bool           `json:"native_windows"`
	WiFiAPI       bool           `json:"wifi_api_available"`
	WiFiAdapter   bool           `json:"wifi_adapter_present"`
	WiFiConnected bool           `json:"wifi_connected"`
	DefaultRoute  bool           `json:"default_ipv4_route_present"`
	RouteKind     ConnectionKind `json:"default_route_kind,omitempty"`
	GatewayICMP   bool           `json:"gateway_icmp_available"`
}

func Capabilities(ctx context.Context, timeout time.Duration) CapabilityReport {
	apis := NewNetworkAPIs()
	report := CapabilityReport{NativeWindows: apis.NativeWindows}
	wifi, wifiErr := apis.WiFi.Snapshot(ctx)
	if wifiErr == nil || errors.Is(wifiErr, ErrNoWiFiAdapter) {
		report.WiFiAPI = apis.NativeWindows
	}
	if wifiErr == nil {
		report.WiFiAdapter, report.WiFiConnected = wifi.AdapterPresent, wifi.Connected
	}
	route, routeErr := apis.Route.DefaultIPv4Route(ctx)
	if routeErr == nil {
		report.DefaultRoute, report.RouteKind = true, route.Kind
		if _, err := apis.Echo.Echo(ctx, route.Gateway, timeout); err == nil {
			report.GatewayICMP = true
		}
	}
	return report
}
