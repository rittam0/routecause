package collector

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"whyfi/internal/model"
)

type fakeReader struct {
	load             float64
	available, total uint64
	err              error
}

func (f fakeReader) CPUCount() int                   { return 4 }
func (f fakeReader) LoadAverage() (float64, error)   { return f.load, f.err }
func (f fakeReader) Memory() (uint64, uint64, error) { return f.available, f.total, f.err }

func TestDeviceProbeUsesInjectedReader(t *testing.T) {
	p := NewDevice(fakeReader{load: 2, available: 25, total: 100}, RealClock{}, func() string { return "id" })
	got := p.Collect(context.Background())
	if !got.Success || got.Values["load_per_core"] != .5 || got.Values["memory_available_percent"] != 25 {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestErrorCategoriesRemainDistinct(t *testing.T) {
	if ClassifyError(context.Canceled) != model.ErrorCancelled {
		t.Fatal("cancellation misclassified")
	}
	if ClassifyError(context.DeadlineExceeded) != model.ErrorTimeout {
		t.Fatal("timeout misclassified")
	}
	if ClassifyError(errors.New("plain")) != model.ErrorSystem {
		t.Fatal("system error misclassified")
	}
}

func TestDeviceProbeHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewDevice(fakeReader{}, RealClock{}, func() string { return "id" })
	got := p.Collect(ctx)
	if got.ErrorCategory != model.ErrorCancelled {
		t.Fatalf("got %s", got.ErrorCategory)
	}
	if got.DurationMS > float64(time.Second/time.Millisecond) {
		t.Fatal("cancelled probe returned too slowly")
	}
}

func TestUnsupportedDeviceProbeIsTypedAndOneShot(t *testing.T) {
	probe := NewUnsupported("device_resources", "local_device", RealClock{}, func() string { return "id" })
	oneShot, ok := probe.(OneShotProbe)
	if !ok || !oneShot.OneShot() {
		t.Fatal("unsupported device collector must be a one-shot availability result")
	}
	got := probe.Collect(context.Background())
	if got.Success || got.ErrorCategory != model.ErrorUnsupported || got.CollectorType != "device_resources" {
		t.Fatalf("unexpected unsupported result: %#v", got)
	}
}

func TestSuccessfulResultAfterDeadlineIsTimeout(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	defer cancel()
	b := base{clock: RealClock{}}
	got := b.finishWithContext(ctx, model.ProbeResult{}, time.Now(), nil)
	if got.Success || got.ErrorCategory != model.ErrorTimeout {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestSuccessfulResultBeforeDeadlineRemainsSuccessful(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	b := base{clock: RealClock{}}
	got := b.finishWithContext(ctx, model.ProbeResult{}, time.Now(), nil)
	if !got.Success {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestHTTPSUnexpectedStatusPreservesTransportTiming(t *testing.T) {
	p := NewHTTPS("https://test.invalid/health", http.MethodGet, []int{http.StatusOK}, RealClock{}, func() string { return "id" }).(*HTTPSProbe)
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxHTTPSBodyBytes+1024))), Header: make(http.Header), Request: r}, nil
	})}
	got := p.Collect(context.Background())
	if !got.Success || got.TransportSuccess == nil || !*got.TransportSuccess {
		t.Fatalf("completed transport was lost: %#v", got)
	}
	if got.EndpointSuccess == nil || *got.EndpointSuccess {
		t.Fatalf("404 was labelled expected: %#v", got)
	}
	if got.Values["status_code"] != 404 || got.Values["total_ms"] <= 0 || got.Values["body_truncated"] != 1 {
		t.Fatalf("timing/status/body evidence missing: %#v", got.Values)
	}
}

func TestHTTPSExpectedStatusAndMethod(t *testing.T) {
	p := NewHTTPS("https://test.invalid/health", http.MethodHead, []int{http.StatusNoContent}, RealClock{}, func() string { return "id" }).(*HTTPSProbe)
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s", r.Method)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: r}, nil
	})}
	got := p.Collect(context.Background())
	if !got.Success || got.EndpointSuccess == nil || !*got.EndpointSuccess || got.HTTPMethod != http.MethodHead {
		t.Fatalf("unexpected result: %#v", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
