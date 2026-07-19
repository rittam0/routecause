# Architecture

## Data flow

`cmd/whyfi` validates flags and handles process signals. The directory name is a temporarily retained internal compatibility name; release executables and user-facing commands are `routecause`. `internal/session` creates a fixed worker pool, schedules independent probe attempts, applies both a session deadline and a deadline to every attempt, and retains raw results. It then computes target-specific summaries and passes only those typed summaries to `internal/diagnosis`.

`internal/collector` contains unprivileged DNS, TCP, HTTPS, device-resource, native Wi-Fi, and gateway probes. A probe performs exactly one attempt. Linux device resources use `/proc`; Windows and other platforms return one typed unsupported device-resource result rather than repeated system errors. `internal/platform` exposes small injected `WiFiAPI`, `RouteAPI`, and `EchoAPI` interfaces. Windows implementations use Native WLAN, IP Helper, and ICMP APIs behind build constraints; non-Windows local-network implementations return typed unsupported results.

The shared domain types live in `internal/model`. This additional package prevents collectors, summaries, and diagnosis from depending on the CLI or on each other merely to exchange data.

## Local product interface

`web/` contains the React/TypeScript Vite source and no Go package. Its native production output is written to `internal/webassets/dist` and embedded with `go:embed`. `routecause serve` listens only on `127.0.0.1`, serves the immutable frontend and `/api/v1`, and opens the fixed local URL in the default browser. The CLI path remains independent. The separate portfolio build compiles a deterministic in-browser fixture adapter to `web/dist-portfolio`; it never calls or claims access to native collectors.

`internal/webapp` owns one active snapshot record, a ten-item in-memory summary history, privacy sanitization, deterministic demo fixtures, and replayable SSE events. A start event contains five waiting network layers; the first typed collector result moves its layer to testing; completion derives factual layer states and observations from summaries. No percentage exists because collectors use different schedules.

SSE events have monotonically increasing per-session IDs. A reconnect with `Last-Event-ID` replays later events, so refresh does not restart or duplicate a diagnosis. Simultaneous starts receive HTTP 409. Cancellation and process shutdown cancel the engine context.

The API enforces a small JSON body limit, rejects unknown fields and invalid duration/demo values, caps SSE clients, and validates Host and Origin against the actual loopback listener. No API field accepts executable names, commands, URLs, DNS names, or probe targets.

## Concurrency and cancellation

One bounded job queue feeds a fixed number of workers. Independent per-probe schedulers use collector-specific intervals and stagger their first attempts over at most one second. Device/Wi-Fi sampling defaults to 1 second, DNS/TCP/gateway to 2 seconds, and HTTPS to 10 seconds. A slow attempt occupies at most one worker. Native ICMP receives the remaining context deadline as its bounded wait.

Actual start times may be later than nominal times when workers are occupied; raw timestamps preserve what happened. A collector returning nil after its context has expired is classified as timeout, closing the deadline-return race. A successful operation just before the deadline remains successful and can be represented as a robust latency outlier.

## Summaries and endpoint roles

Reliability is represented by success, failure, and timeout rates plus the longest consecutive failure run. Typical performance uses the median. Tail instability uses sample-gated p95/p99 and robust outliers based on the Hampel rule (`|x - median| > 3 × 1.4826 × MAD`). Sustained degradation, severe tails, repeated failures, and isolated events are therefore distinct typed signals. Mean, minimum, and maximum are descriptive only.

p95 requires 20 successful samples and p99 requires 100. At exactly those counts the tail contains only about one observation, so percentile estimates are coarse and must not be interpreted as stable population percentiles. Lower-frequency HTTPS commonly has no percentile in a single 30-second session.

HTTPS targets have explicit primary-health and independent-secondary roles and default to small responses, rather than ordinary homepages whose rendering/backend cost dominated the network signal. Each role configures URL, `HEAD`/`GET` method, and expected status codes. GET reads are bounded and discarded. A completed HTTP exchange is transport success even when its status is unexpected; endpoint success is tracked separately, so a 404 retains timing evidence without satisfying endpoint-health diagnosis. DNS and TCP have separate replaceable targets. Public endpoints are imperfect: operators may change behavior or apply rate limits.

## Snapshot interpretation boundaries

The engine remains a deterministic function of `[]CollectorSummary` and configured thresholds, and stable diagnosis codes remain in the schema for compatibility. Public conclusions are bounded observations: no significant problem in this test, repeated failures in one measured layer, or a brief interruption that the snapshot could not isolate. They never assign causation to a router, ISP or device.

A native local diagnosis requires an unambiguous active route and sufficient gateway evidence. Wi-Fi attribution additionally requires the active route to be Wi-Fi. Weak/fluctuating Wi-Fi, disconnect/reconnect activity, gateway loss, sustained median latency, consecutive failures, and independent public target behavior are evaluated as deterministic typed patterns. Gateway p95 below the configured slow threshold is healthy, and robust statistical outliers alone never change the diagnosis. Repeated above-threshold tail latency without corroboration is retained only as an advanced qualification. Upstream requires a stable local path and two independently operated degraded target groups; one endpoint can never establish it. VPN/tunnel routes, equal default routes, unavailable measurements, and unclear patterns return `unknown`.

Linux and WSL retain milestone-one symptom behavior because native collectors are explicitly unsupported. No traceroute inference exists, and a silent hop cannot be treated as an ISP fault.

## Privacy and persistence

RouteCause writes configured targets, timings, coarse OS/architecture context, status codes, sanitized errors, session-local labels, radio family, signal-quality percentage, and negotiated link rates. Native adapters may return raw SSID internally, but the collector never copies it into persisted results. MAC addresses, BSSIDs, interface GUIDs/descriptions, and gateway IP addresses are not persisted. The capability command reports booleans and route kind only.

The browser interface adds a defense-in-depth sanitizer before retrieval/export and stores nothing in cookies, local storage, or session storage. Local history retains only ID, timestamps, requested duration, completion status, typed diagnosis/confidence, demo label, and final layer states. There is no database, upload, login, telemetry, or analytics.
