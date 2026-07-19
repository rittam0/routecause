package webassets

import (
	"io/fs"
	"strings"
	"testing"
)

func TestProductionInterfaceIsEmbedded(t *testing.T) {
	assets, err := Assets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "RouteCause") || !strings.Contains(string(index), "/assets/") {
		t.Fatalf("unexpected embedded index: %s", index)
	}
	entries, err := fs.ReadDir(assets, "assets")
	if err != nil || len(entries) < 2 {
		t.Fatalf("embedded production assets missing: %v %#v", err, entries)
	}
}
