//go:build !windows

package platform

import (
	"context"
	"net"
	"time"
)

type unsupportedWiFi struct{}

func (unsupportedWiFi) Snapshot(context.Context) (WiFiSnapshot, error) {
	return WiFiSnapshot{}, ErrUnsupported
}

type unsupportedRoute struct{}

func (unsupportedRoute) DefaultIPv4Route(context.Context) (DefaultRoute, error) {
	return DefaultRoute{}, ErrUnsupported
}

type unsupportedEcho struct{}

func (unsupportedEcho) Echo(context.Context, net.IP, time.Duration) (EchoReply, error) {
	return EchoReply{}, ErrUnsupported
}

func newNetworkAPIs() NetworkAPIs {
	return NetworkAPIs{WiFi: unsupportedWiFi{}, Route: unsupportedRoute{}, Echo: unsupportedEcho{}, NativeWindows: false}
}
