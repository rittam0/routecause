package collector

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"whyfi/internal/model"
	"whyfi/internal/platform"
)

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type IDGenerator func() string

type Probe interface {
	Type() string
	Target() string
	Collect(context.Context) model.ProbeResult
}

// OneShotProbe identifies availability results that should be collected once
// rather than repeated throughout a session.
type OneShotProbe interface {
	Probe
	OneShot() bool
}

type base struct {
	kind, target string
	clock        Clock
	id           IDGenerator
}

func (b base) Type() string   { return b.kind }
func (b base) Target() string { return b.target }
func (b base) start() model.ProbeResult {
	return model.ProbeResult{ID: b.id(), CollectorType: b.kind, Target: b.target, StartedAt: b.clock.Now(), OSContext: map[string]string{"os": runtime.GOOS, "arch": runtime.GOARCH}}
}
func (b base) finish(r model.ProbeResult, started time.Time, err error) model.ProbeResult {
	r.DurationMS = float64(b.clock.Now().Sub(started).Microseconds()) / 1000
	if err == nil {
		r.Success = true
		return r
	}
	r.ErrorCategory, r.ErrorMessage = ClassifyError(err), safeError(err)
	return r
}

// finishWithContext closes the small race where an operation returns a nil error
// just after its context deadline. A result completed before the deadline remains
// successful, even when it is close to the configured timeout.
func (b base) finishWithContext(ctx context.Context, r model.ProbeResult, started time.Time, err error) model.ProbeResult {
	if err == nil {
		err = ctx.Err()
	}
	return b.finish(r, started, err)
}

func ClassifyError(err error) model.ErrorCategory {
	if err == nil {
		return model.ErrorNone
	}
	if errors.Is(err, context.Canceled) {
		return model.ErrorCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return model.ErrorTimeout
	}
	if errors.Is(err, platform.ErrUnsupported) {
		return model.ErrorUnsupported
	}
	if errors.Is(err, platform.ErrNoWiFiAdapter) {
		return model.ErrorNoAdapter
	}
	if errors.Is(err, platform.ErrNotConnected) {
		return model.ErrorDisconnected
	}
	if errors.Is(err, platform.ErrRouteAmbiguous) {
		return model.ErrorAmbiguous
	}
	if errors.Is(err, platform.ErrICMPUnavailable) {
		return model.ErrorICMPBlocked
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsTimeout {
			return model.ErrorTimeout
		}
		return model.ErrorDNS
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return model.ErrorTimeout
		}
		if strings.Contains(strings.ToLower(opErr.Err.Error()), "refused") {
			return model.ErrorRefused
		}
	}
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) {
		return model.ErrorTLS
	}
	return model.ErrorSystem
}

func safeError(err error) string {
	category := ClassifyError(err)
	switch category {
	case model.ErrorTimeout:
		return "operation timed out"
	case model.ErrorCancelled:
		return "operation cancelled"
	case model.ErrorRefused:
		return "connection refused"
	case model.ErrorDNS:
		return "DNS lookup failed"
	case model.ErrorTLS:
		return "TLS handshake failed"
	default:
		return "operation failed"
	}
}

type DNSProbe struct {
	base
	resolver *net.Resolver
	hostname string
}

func NewSystemDNS(hostname string, clock Clock, id IDGenerator) Probe {
	return &DNSProbe{base: base{kind: "dns_system", target: hostname, clock: clock, id: id}, resolver: net.DefaultResolver, hostname: hostname}
}

func NewComparisonDNS(hostname, address string, clock Clock, id IDGenerator) Probe {
	resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", address)
	}}
	return &DNSProbe{base: base{kind: "dns_comparison", target: address + "/" + hostname, clock: clock, id: id}, resolver: resolver, hostname: hostname}
}

func (p *DNSProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	started := p.clock.Now()
	addresses, err := p.resolver.LookupHost(ctx, p.hostname)
	r.Values = map[string]float64{"answer_count": float64(len(addresses)), "dns_ms": float64(p.clock.Now().Sub(started).Microseconds()) / 1000}
	return p.finishWithContext(ctx, r, started, err)
}

type TCPProbe struct {
	base
	dialer *net.Dialer
}

func NewTCP(target string, clock Clock, id IDGenerator) Probe {
	return &TCPProbe{base: base{kind: "tcp_connect", target: target, clock: clock, id: id}, dialer: &net.Dialer{}}
}
func (p *TCPProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	started := p.clock.Now()
	conn, err := p.dialer.DialContext(ctx, "tcp", p.target)
	if conn != nil {
		_ = conn.Close()
	}
	r.Values = map[string]float64{"connect_ms": float64(p.clock.Now().Sub(started).Microseconds()) / 1000}
	return p.finishWithContext(ctx, r, started, err)
}

type HTTPSProbe struct {
	base
	client           *http.Client
	method           string
	expectedStatuses map[int]struct{}
}

const maxHTTPSBodyBytes = 64 * 1024

func NewHTTPS(target, method string, expectedStatuses []int, clock Clock, id IDGenerator) Probe {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, TLSHandshakeTimeout: 0, ResponseHeaderTimeout: 0}
	expected := make(map[int]struct{}, len(expectedStatuses))
	for _, status := range expectedStatuses {
		expected[status] = struct{}{}
	}
	return &HTTPSProbe{base: base{kind: "https", target: target, clock: clock, id: id}, client: &http.Client{Transport: transport}, method: strings.ToUpper(method), expectedStatuses: expected}
}
func (p *HTTPSProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	r.HTTPMethod = p.method
	started := p.clock.Now()
	values := map[string]float64{}
	var dnsStart, connectStart, tlsStart time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { dnsStart = p.clock.Now() }, DNSDone: func(httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				values["dns_ms"] = ms(p.clock.Now().Sub(dnsStart))
			}
		},
		ConnectStart: func(_, _ string) { connectStart = p.clock.Now() }, ConnectDone: func(_, _ string, _ error) {
			if !connectStart.IsZero() {
				values["connect_ms"] = ms(p.clock.Now().Sub(connectStart))
			}
		},
		TLSHandshakeStart: func() { tlsStart = p.clock.Now() }, TLSHandshakeDone: func(tls.ConnectionState, error) {
			if !tlsStart.IsZero() {
				values["tls_ms"] = ms(p.clock.Now().Sub(tlsStart))
			}
		},
		GotFirstResponseByte: func() { values["ttfb_ms"] = ms(p.clock.Now().Sub(started)) },
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), p.method, p.target, nil)
	if err != nil {
		return p.finish(r, started, err)
	}
	req.Header.Set("User-Agent", "routecause/1")
	resp, err := p.client.Do(req)
	if resp != nil {
		values["status_code"] = float64(resp.StatusCode)
		if p.method == http.MethodGet && err == nil {
			read, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxHTTPSBodyBytes+1))
			values["body_bytes_read"] = float64(read)
			if read > maxHTTPSBodyBytes {
				values["body_truncated"] = 1
			}
			if readErr != nil {
				err = readErr
			}
		}
		_ = resp.Body.Close()
	}
	values["total_ms"] = ms(p.clock.Now().Sub(started))
	r.Values = values
	result := p.finishWithContext(ctx, r, started, err)
	transportSuccess := result.Success
	result.TransportSuccess = &transportSuccess
	if transportSuccess && resp != nil {
		_, endpointSuccess := p.expectedStatuses[resp.StatusCode]
		result.EndpointSuccess = &endpointSuccess
	}
	return result
}
func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

type SystemReader interface {
	CPUCount() int
	LoadAverage() (float64, error)
	Memory() (available, total uint64, err error)
}
type ProcSystemReader struct{}

func (ProcSystemReader) CPUCount() int { return runtime.NumCPU() }
func (ProcSystemReader) LoadAverage() (float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("empty loadavg")
	}
	return strconv.ParseFloat(fields[0], 64)
}
func (ProcSystemReader) Memory() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	var available, total uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = value * 1024
		case "MemAvailable:":
			available = value * 1024
		}
	}
	if total == 0 {
		return 0, 0, errors.New("MemTotal unavailable")
	}
	return available, total, nil
}

type DeviceProbe struct {
	base
	reader SystemReader
}

type UnsupportedProbe struct{ base }

func NewUnsupported(kind, target string, clock Clock, id IDGenerator) Probe {
	return &UnsupportedProbe{base: base{kind: kind, target: target, clock: clock, id: id}}
}

func (*UnsupportedProbe) OneShot() bool { return true }
func (p *UnsupportedProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	started := p.clock.Now()
	if err := ctx.Err(); err != nil {
		return p.finish(r, started, err)
	}
	return p.finish(r, started, platform.ErrUnsupported)
}

func NewDevice(reader SystemReader, clock Clock, id IDGenerator) Probe {
	return &DeviceProbe{base: base{kind: "device_resources", target: "local_device", clock: clock, id: id}, reader: reader}
}
func (p *DeviceProbe) Collect(ctx context.Context) model.ProbeResult {
	r := p.start()
	started := p.clock.Now()
	if err := ctx.Err(); err != nil {
		return p.finish(r, started, err)
	}
	load, err := p.reader.LoadAverage()
	if err != nil {
		return p.finish(r, started, err)
	}
	available, total, err := p.reader.Memory()
	if err != nil {
		return p.finish(r, started, err)
	}
	cores := p.reader.CPUCount()
	r.Values = map[string]float64{"cpu_count": float64(cores), "load_1m": load, "load_per_core": load / float64(cores), "memory_available_percent": float64(available) / float64(total) * 100}
	return p.finish(r, started, nil)
}
