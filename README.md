# RouteCause

RouteCause is a private, local-first 60-second network snapshot that measures each connection layer and reports what happened during the test.

> **Screenshot placeholder:** release UI capture pending on a browser-capable environment.

[Live Demo](#live-demo) · [Windows Download](#windows-download-and-native-run) · [Architecture](docs/architecture.md) · [Native Windows validation](docs/validation/native-windows.md)

**Verified benchmark:** the local HTTP/SSE layer completed streams for 10,000 simulated clients with 0% observed errors in the recorded WSL run. These were simulated clients, explicitly not users, and the result is not a capacity guarantee. [Method and measurements](docs/benchmarks/milestone2-wsl.md).

## Quick start

```sh
cd web
npm ci
npm run build
cd ..
make build
./bin/routecause serve
```

RouteCause measures Wi-Fi or Ethernet, the local gateway, DNS, the internet path and configured destinations. It does not claim to prove the underlying root cause. The primary interface offers one full 60-second snapshot and a privacy-safe JSON export.

Results are observations, not fault ownership: healthy, issue observed, not measured or insufficient data for each layer. “No significant problem was observed during this test” says nothing about behavior outside the measurement window.

Release screenshots and recording are intentionally not checked in yet: this WSL image lacks Chromium's `libnspr4.so` runtime dependency, and no compatible system browser is installed. The exact deterministic capture commands are in the verification section below; generated filenames are `docs/assets/routecause-start.png`, `routecause-result.png` and `routecause-demo.webm`.

## Product architecture

- A Go diagnostic engine collects typed DNS, TCP, HTTPS, device, Windows Wi-Fi and gateway evidence.
- A React and TypeScript interface is built by Vite and embedded into the Go executable.
- The native application listens only on `127.0.0.1`, opens the local interface and streams collector state over replayable Server-Sent Events.
- One snapshot runs at a time. Refreshing reconnects to the active session rather than starting another.
- The CLI remains available for scripted diagnoses and capability inspection.
- No account, cloud service, analytics, database or upload path exists.

API endpoints are versioned under `/api/v1`: capabilities, diagnosis start, replayable events, completed-session retrieval, cancellation, JSON export and privacy-safe in-memory history.

See [docs/architecture.md](docs/architecture.md), [the interface design](docs/product-interface-design.md) and [ADR 0004](docs/decisions/0004-embedded-local-product-interface.md).

## Build and run

Go 1.26 or newer and Node/npm compatible with the locked frontend dependencies are expected for development.

```sh
cd web
npm ci
npm run build
cd ..
make build
./bin/routecause serve
```

Use `--no-open` when WSL cannot open a browser, then visit `http://127.0.0.1:8765`.

The CLI remains available:

```sh
./bin/routecause capabilities
./bin/routecause diagnose --duration 60s --output routecause-session.json
```

The Go module name `whyfi`, `cmd/whyfi` directory, temporary Go cache path, and historical schema ID `https://whyfi.local/schema/session-1.3.0.json` are retained compatibility identifiers. They are not current product branding. Renaming them would break imports, tooling, or schema identity without improving the user-facing release.

## Live Demo

Native mode runs the real local collectors available on the current platform:

```sh
./bin/routecause serve
```

Executable demo mode serves the same product shell but substitutes deterministic fixtures:

```sh
./bin/routecause serve --demo
```

It is persistently labelled “Interactive Demo” and “no live measurements.” Its seven scenarios demonstrate observed failure patterns; none is presented as a proven cause.

The Vercel-ready static portfolio build is an honest product page rather than a
simulated network test. It explains the browser boundary, presents privacy-safe
aggregate evidence from the three controlled native Windows runs, documents the
systems architecture and directs users to run the native application:

```sh
cd web
npm run build:portfolio
npx vite preview --outDir dist-portfolio
```

Deployable static files are produced in `web/dist-portfolio`. The page makes no
API request and performs no simulated or live diagnosis. It explicitly states
that a browser cannot access native Wi-Fi, routes, gateway ICMP or device
measurements. `web/vercel.json` fixes the portfolio build command, output
directory and SPA rewrite; this repository does not deploy it automatically.

## Windows download and native run

Build the release artifact reproducibly:

```sh
make build-windows
```

Copy `bin/routecause-windows-amd64.exe` to the Windows machine. In PowerShell:

```powershell
.\routecause-windows-amd64.exe capabilities
.\routecause-windows-amd64.exe serve
```

Windows may display its normal warning for an unsigned executable. RouteCause needs no administrator privileges. Native Windows WLAN, IP Helper and ICMP APIs provide physical Wi-Fi and gateway evidence where supported. WSL intentionally reports Windows-native collectors as unsupported.

Device-resource pressure currently uses Linux `/proc` semantics. Native Windows reports that collector unsupported once; it does not fabricate equivalent values or repeatedly emit system errors.

Three controlled native Windows acceptance runs exercised disconnected Wi-Fi,
Wi-Fi with Proton VPN active, and ordinary Wi-Fi. Their privacy-safe aggregate
evidence and explicit interpretation limits are documented in
[docs/validation/native-windows.md](docs/validation/native-windows.md). Raw
reports are intentionally not committed.

## Privacy and security

RouteCause does not persist raw SSIDs, MAC addresses, BSSIDs, interface GUIDs/descriptions or gateway IP addresses. Wi-Fi is represented by a session-local label. Browser storage, external analytics and third-party frontend requests are absent. History contains at most ten privacy-safe summaries in process memory and disappears on exit.

The local server validates Host and Origin, caps request and header sizes, rejects unknown JSON, applies a restrictive Content Security Policy, limits SSE clients, accepts no user-supplied commands or targets, and safely cancels active work during shutdown. Exported CLI files use mode `0600`.

## Verification

Current snapshot-contract pass (2026-07-19): frontend formatting and lint passed; 3 Vitest files with 20 tests passed; native and portfolio production builds passed; all Go tests, race detection and vet passed; and Linux amd64, Windows amd64, local-server and load-harness builds passed. The existing Playwright command could not start its loopback test server in this restricted workspace (`listen tcp 127.0.0.1:8765: socket: operation not permitted`), so browser E2E behavior is not claimed as verified in this pass. The previously recorded staged load run through 10,000 simulated SSE clients remains documented separately and was not recharacterized as users.

Run the release matrix:

```sh
cd web
npm run format
npm run lint
npm test
npm run build
npm run build:portfolio
cd ..
make build
cd web
npm run test:e2e
cd ..
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
make build-linux
make build-windows
```

Playwright covers application load, diagnosis start, real SSE progress, refresh/reconnection, completion, cancellation, export and every labelled demo scenario. Component tests cover all layer and diagnosis states with an axe accessibility check. Go tests cover API lifecycle, privacy, malformed input, negative Host/Origin cases, concurrency, event replay, embedded assets and collector/diagnosis behavior. Automated tests use fixtures rather than the public internet.

To recapture release media, start demo mode in one terminal and capture in another:

```sh
./bin/routecause serve --demo --no-open
cd web
npm run capture:media
```

## Reproducible HTTP/SSE load harness

The harness creates simulated SSE clients, never “users.” Start a controlled demo server with the required client ceiling and note its PID:

```sh
make build load-build
./bin/routecause serve --demo --no-open --max-sse-clients 10000 &
ROUTECAUSE_PID=$!
./bin/routecause-load --pid "$ROUTECAUSE_PID" --stages 10,100,1000,10000 > routecause-load.json
kill "$ROUTECAUSE_PID"
```

Each stage starts one deterministic diagnosis and connects the requested number of simulated clients to its SSE stream. JSON output records connection-latency p50/p95/p99, completed-stream throughput, error rate and completed stream count. On Linux, `--pid` also samples peak server RSS and process CPU seconds from `/proc`; omit it on unsupported systems and those server-process fields remain unavailable. Host file-descriptor, port and memory limits can prevent the 10,000 stage and must be reported as test-environment limits rather than hidden.

## Snapshot model and limitations

Summaries use success/failure/timeout rates, median, MAD, robust Hampel outliers and consecutive failures. p95 appears from 20 successful samples and p99 from 100; these remain coarse session descriptors at their minimum sample sizes. Mean, minimum and maximum are descriptive rather than primary diagnosis inputs.

Internally retained compatibility codes group repeated Wi-Fi, gateway, DNS, internet-path and destination patterns. Public results report only what occurred in the 60-second window and never state that a router, ISP or device caused it. Mixed evidence says the interruption could not be isolated; missing or ambiguous evidence is insufficient data. Silent traceroute hops are never ISP evidence.

Gateway p95 below the configured slow threshold is healthy. Statistical outliers alone remain in the Advanced report and never block a healthy result. Unsupported Windows device-resource collection is neutral and is not one of the five network-layer cards.

Important limitations:

- Public endpoints can rate-limit, redirect, change or fail independently.
- Gateway ICMP behavior can differ from forwarded application traffic.
- Route presence is not represented as successful gateway reachability.
- No throughput test or ISP attribution is performed.
- Quiet sessions cannot exclude intermittent faults.
- Native Wi-Fi/gateway behavior and browser launching were validated on one Windows 11 computer; broader adapter, router, VPN and Windows-version coverage remains outstanding.
- The Windows binary is currently unsigned.
- Session history is memory-only and one diagnosis may run at a time.

## Release checklist

- [x] Frontend format, lint, component, production and portfolio builds pass.
- [ ] Playwright lifecycle, reconnect, cancellation, export and seven-scenario suite passes.
- [x] Go tests, race detector and vet pass.
- [x] Linux and Windows release-named binaries build.
- [x] Embedded UI and all executable demo scenarios are smoke-tested.
- [ ] Static portfolio build is inspected and cannot imply native collection.
- [x] Load stages and environmental limits are recorded as simulated-client results.
- [x] Native Windows capabilities, live snapshot, browser open and export are manually validated across three controlled conditions.
- [ ] Native Windows cancellation is manually validated.
- [ ] Release screenshots and recording match the current bundle.
- [ ] No credentials, private raw results or unintended generated files are included.
