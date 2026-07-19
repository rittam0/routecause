package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCapabilitiesDoesNotExposePrivateIdentifiers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"capabilities"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(stdout.String())
	for _, forbidden := range []string{"ssid", "bssid", "guid", "mac_address", "gateway_address", "interface_name"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("capability output exposed %q: %s", forbidden, text)
		}
	}
}
