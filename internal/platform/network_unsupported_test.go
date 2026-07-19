//go:build !windows

package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNonWindowsNativeNetworkEvidenceIsTypedUnsupported(t *testing.T) {
	apis := NewNetworkAPIs()
	if apis.NativeWindows {
		t.Fatal("non-Windows build claimed native Windows evidence")
	}
	if _, err := apis.WiFi.Snapshot(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Wi-Fi error = %v", err)
	}
	if _, err := apis.Route.DefaultIPv4Route(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("route error = %v", err)
	}
	if report := Capabilities(context.Background(), time.Millisecond); report.NativeWindows || report.WiFiAdapter || report.DefaultRoute {
		t.Fatalf("fabricated capability: %#v", report)
	}
}
