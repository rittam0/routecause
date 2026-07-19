package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"whyfi/internal/browser"
	"whyfi/internal/collector"
	"whyfi/internal/config"
	"whyfi/internal/model"
	"whyfi/internal/platform"
	"whyfi/internal/session"
	"whyfi/internal/webapp"
	"whyfi/internal/webassets"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "routecause:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "serve" {
		return runServe(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "capabilities" {
		report := platform.Capabilities(context.Background(), time.Second)
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if len(args) == 0 || args[0] != "diagnose" {
		return errors.New("usage: routecause diagnose [options] | routecause serve [options] | routecause capabilities")
	}
	cfg := config.Default()
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "session.json", "path for the diagnostic session JSON")
	flags.DurationVar(&cfg.Duration, "duration", cfg.Duration, "total diagnostic duration")
	flags.DurationVar(&cfg.DeviceInterval, "device-interval", cfg.DeviceInterval, "interval between device resource measurements")
	flags.DurationVar(&cfg.DNSInterval, "dns-interval", cfg.DNSInterval, "interval between DNS measurements")
	flags.DurationVar(&cfg.TCPInterval, "tcp-interval", cfg.TCPInterval, "interval between TCP measurements")
	flags.DurationVar(&cfg.HTTPSInterval, "https-interval", cfg.HTTPSInterval, "interval between HTTPS measurements")
	flags.DurationVar(&cfg.WiFiInterval, "wifi-interval", cfg.WiFiInterval, "interval between native Wi-Fi measurements")
	flags.DurationVar(&cfg.GatewayInterval, "gateway-interval", cfg.GatewayInterval, "interval between gateway measurements")
	flags.DurationVar(&cfg.ProbeTimeout, "probe-timeout", cfg.ProbeTimeout, "strict timeout for each probe")
	flags.StringVar(&cfg.DNSHostname, "dns-hostname", cfg.DNSHostname, "hostname used for DNS timing")
	resolvers := flags.String("resolvers", strings.Join(cfg.ComparisonResolvers, ","), "comma-separated DNS resolver host:port values")
	tcpTargets := flags.String("tcp-targets", strings.Join(cfg.TCPDestinations, ","), "comma-separated TCP host:port destinations")
	flags.StringVar(&cfg.HTTPSHealthEndpoint, "https-health-endpoint", cfg.HTTPSHealthEndpoint, "lightweight primary HTTPS health URL")
	flags.StringVar(&cfg.HTTPSSecondaryEndpoint, "https-secondary-endpoint", cfg.HTTPSSecondaryEndpoint, "independent secondary HTTPS URL")
	flags.StringVar(&cfg.HTTPSHealthMethod, "https-health-method", cfg.HTTPSHealthMethod, "HTTP method for primary health endpoint (HEAD or GET)")
	healthStatuses := flags.String("https-health-statuses", joinInts(cfg.HTTPSHealthStatuses), "comma-separated expected primary endpoint status codes")
	flags.StringVar(&cfg.HTTPSSecondaryMethod, "https-secondary-method", cfg.HTTPSSecondaryMethod, "HTTP method for secondary endpoint (HEAD or GET)")
	secondaryStatuses := flags.String("https-secondary-statuses", joinInts(cfg.HTTPSSecondaryStatuses), "comma-separated expected secondary endpoint status codes")
	flags.IntVar(&cfg.MaxConcurrency, "max-concurrency", cfg.MaxConcurrency, "maximum concurrent probes")
	flags.IntVar(&cfg.Thresholds.MinimumSamples, "minimum-samples", cfg.Thresholds.MinimumSamples, "successful samples required for a conclusion")
	flags.Float64Var(&cfg.Thresholds.DNSSlowMS, "dns-slow-ms", cfg.Thresholds.DNSSlowMS, "system DNS degradation threshold")
	flags.Float64Var(&cfg.Thresholds.DNSHealthyMS, "dns-healthy-ms", cfg.Thresholds.DNSHealthyMS, "comparison DNS healthy threshold")
	flags.Float64Var(&cfg.Thresholds.ConnectSlowMS, "connect-slow-ms", cfg.Thresholds.ConnectSlowMS, "slow TCP connection threshold")
	flags.Float64Var(&cfg.Thresholds.ConnectHealthyMS, "connect-healthy-ms", cfg.Thresholds.ConnectHealthyMS, "healthy TCP connection threshold")
	flags.Float64Var(&cfg.Thresholds.HTTPSTotalSlowMS, "https-slow-ms", cfg.Thresholds.HTTPSTotalSlowMS, "slow HTTPS total threshold")
	flags.Float64Var(&cfg.Thresholds.CPULoadPerCoreHigh, "cpu-load-high", cfg.Thresholds.CPULoadPerCoreHigh, "high one-minute load per logical CPU")
	flags.Float64Var(&cfg.Thresholds.MemoryAvailableLowPC, "memory-available-low-percent", cfg.Thresholds.MemoryAvailableLowPC, "low available memory threshold")
	flags.Float64Var(&cfg.Thresholds.WiFiWeakPercent, "wifi-weak-percent", cfg.Thresholds.WiFiWeakPercent, "weak Wi-Fi signal-quality threshold")
	flags.Float64Var(&cfg.Thresholds.GatewaySlowMS, "gateway-slow-ms", cfg.Thresholds.GatewaySlowMS, "slow local gateway latency threshold")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	cfg.ComparisonResolvers, cfg.TCPDestinations = split(*resolvers), split(*tcpTargets)
	var err error
	if cfg.HTTPSHealthStatuses, err = splitInts(*healthStatuses); err != nil {
		return fmt.Errorf("invalid health statuses: %w", err)
	}
	if cfg.HTTPSSecondaryStatuses, err = splitInts(*secondaryStatuses); err != nil {
		return fmt.Errorf("invalid secondary statuses: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	diagnostic, err := session.NewRunner(cfg, collector.RealClock{}, logger).Run(ctx)
	if err != nil {
		return err
	}
	if err := writeJSON(*output, diagnostic); err != nil {
		return err
	}
	printConclusion(stdout, diagnostic, *output)
	return nil
}

func runServe(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	port := flags.Int("port", 8765, "local interface port")
	quick := flags.Duration("quick-duration", 15*time.Second, "quick diagnosis duration")
	full := flags.Duration("full-duration", 60*time.Second, "full diagnosis duration")
	demo := flags.Bool("demo", false, "use deterministic demo fixtures instead of live measurements")
	noOpen := flags.Bool("no-open", false, "do not open the default browser")
	maxSSEClients := flags.Int("max-sse-clients", 8, "maximum concurrent SSE clients (raise only for controlled load tests)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *port < 0 || *port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	if *quick < 5*time.Second || *full < *quick || *full > 10*time.Minute {
		return errors.New("durations must satisfy 5s <= quick <= full <= 10m")
	}
	if *maxSSEClients < 1 || *maxSSEClients > 10000 {
		return errors.New("max-sse-clients must be between 1 and 10000")
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return fmt.Errorf("listen on localhost: %w", err)
	}
	defer listener.Close()
	if !webapp.IsLoopbackAddress(listener.Addr().String()) {
		return errors.New("refusing to serve on a non-loopback address")
	}
	assets, err := webassets.Assets()
	if err != nil {
		return fmt.Errorf("load embedded interface: %w", err)
	}
	base := config.Default()
	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	manager := webapp.NewManager(webapp.EngineRunner{Base: base, Logger: logger}, webapp.DemoRunner{}, *demo, *quick, *full)
	host := listener.Addr().String()
	_, actualPort, _ := net.SplitHostPort(host)
	app := webapp.NewServer(webapp.ServerOptions{Manager: manager, Assets: assets, AllowedHosts: []string{host, "localhost:" + actualPort}, DemoMode: *demo, Quick: *quick, Full: *full, MaxSSEClients: int32(*maxSSEClients)})
	httpServer := &http.Server{Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- httpServer.Serve(listener) }()
	url := "http://" + host + "/"
	fmt.Fprintf(stdout, "RouteCause is available at %s\n", url)
	if *demo {
		fmt.Fprintln(stdout, "DEMO MODE: deterministic fixtures; no live diagnosis will run.")
	}
	if !*noOpen {
		if err := browser.Open(url); err != nil {
			logger.Warn("could not open browser", "error", err)
		}
	}
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		manager.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown local interface: %w", err)
		}
	}
	return nil
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}

func splitInts(value string) ([]int, error) {
	out := []int{}
	for _, part := range split(value) {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("%q is not a status code", part)
		}
		out = append(out, parsed)
	}
	return out, nil
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
func writeJSON(path string, value model.DiagnosticSession) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
func printConclusion(w io.Writer, s model.DiagnosticSession, path string) {
	fmt.Fprintf(w, "RouteCause test summary: %s\n", s.Diagnosis.Conclusion)
	fmt.Fprintln(w, "This snapshot reports observations from the test window; it does not prove root cause.")
	if s.Diagnosis.Confidence != "" {
		fmt.Fprintf(w, "Confidence: %s\n", s.Diagnosis.Confidence)
	} else {
		fmt.Fprintln(w, "Confidence: withheld (insufficient evidence)")
	}
	if len(s.Diagnosis.SupportingEvidence) > 0 {
		fmt.Fprintln(w, "Evidence:")
		for _, evidence := range s.Diagnosis.SupportingEvidence {
			fmt.Fprintf(w, "  - %s\n", evidence.Statement)
		}
	}
	if len(s.Diagnosis.Alternatives) > 0 {
		fmt.Fprintln(w, "Alternatives:")
		for _, alternative := range s.Diagnosis.Alternatives {
			fmt.Fprintf(w, "  - %s\n", alternative)
		}
	}
	fmt.Fprintf(w, "Raw session: %s (%s)\n", path, time.Duration(s.EndedAt.Sub(s.StartedAt)).Round(time.Millisecond))
}
