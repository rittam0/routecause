# RouteCause

RouteCause is a private, local-first diagnostic that identifies where a slow or unstable connection is most likely failing instead of reporting only a speed.

> **Screenshot placeholder:** release UI capture pending on a browser-capable environment.

[Live Demo](#live-demo) · [Windows Download](#windows-download-and-native-run) · [Architecture](docs/architecture.md)

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

RouteCause tests the supported path from device resources through native Wi-Fi or Ethernet, the local gateway, DNS and independent destinations. The result is a bounded likely cause with evidence, contradictions, alternatives, limitations and a privacy-safe JSON export.

RouteCause does not claim fault ownership. “Likely upstream degradation” is not proof of an ISP fault, and “No material problem was observed during this session” is not a guarantee that the connection is healthy outside the measurement window.

Release screenshots and recording are intentionally not checked in yet: this WSL image lacks Chromium's `libnspr4.so` runtime dependency, and no compatible system browser is installed. The exact deterministic capture commands are in the verification section below; generated filenames are `docs/assets/routecause-start.png`, `routecause-result.png` and `routecause-demo.webm`.

## Product architecture

- A Go diagnostic engine collects typed DNS, TCP, HTTPS, device, Windows Wi-Fi and gateway evidence.
- A React and TypeScript interface is built by Vite and embedded into the Go executable.
- The native application listens only on `127.0.0.1`, opens the local interface and streams collector state over replayable Server-Sent Events.
- One diagnosis runs at a time. Refreshing reconnects to the active session rather than starting another.
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

It is persistently labelled “Interactive Demo” and “no live measurements.” Available scenarios are healthy, DNS degradation, local Wi-Fi instability, gateway instability, upstream degradation, destination-specific degradation and unknown.

The Vercel-ready static portfolio build uses those fixtures entirely in the browser:

```sh
cd web
npm run build:portfolio
npx vite preview --outDir dist-portfolio
```

Deployable static files are produced in `web/dist-portfolio`. The static interface explicitly states that a browser cannot access native Wi-Fi, routes, gateway ICMP or device measurements. It never falls back to public probes and cannot be confused with a native diagnosis. `web/vercel.json` supplies the SPA rewrite; this repository does not deploy it.

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

## Privacy and security

RouteCause does not persist raw SSIDs, MAC addresses, BSSIDs, interface GUIDs/descriptions or gateway IP addresses. Wi-Fi is represented by a session-local label. Browser storage, external analytics and third-party frontend requests are absent. History contains at most ten privacy-safe summaries in process memory and disappears on exit.

The local server validates Host and Origin, caps request and header sizes, rejects unknown JSON, applies a restrictive Content Security Policy, limits SSE clients, accepts no user-supplied commands or targets, and safely cancels active work during shutdown. Exported CLI files use mode `0600`.

## Verification

Current release-readiness pass (2026-07-19): frontend formatting and lint passed; 3 Vitest files with 17 tests passed; native and portfolio production builds passed; all Go tests, race detection and vet passed; Linux amd64 and Windows amd64 cross-builds passed; all seven executable demo scenarios returned their expected typed codes; and staged load runs completed through 10,000 simulated SSE clients with zero observed errors. The 11 Playwright tests were discovered and the embedded server started, but Chromium execution is blocked in this WSL image by missing `libnspr4.so`; screenshots and recording share that blocker and are not claimed as verified.

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

## Diagnostic model and limitations

Summaries use success/failure/timeout rates, median, MAD, robust Hampel outliers and consecutive failures. p95 appears from 20 successful samples and p99 from 100; these remain coarse session descriptors at their minimum sample sizes. Mean, minimum and maximum are descriptive rather than primary diagnosis inputs.

Native Windows rules distinguish likely physical Wi-Fi instability, local gateway instability, degradation beyond a stable local path, and one-destination degradation. Missing, contradictory, VPN-routed or ambiguous evidence returns unknown. Two independent public target groups are required before upstream degradation is considered. Silent traceroute hops are never ISP evidence.

Important limitations:

- Public endpoints can rate-limit, redirect, change or fail independently.
- Gateway ICMP behavior can differ from forwarded application traffic.
- Route presence is not represented as successful gateway reachability.
- No throughput test or ISP attribution is performed.
- Quiet sessions cannot exclude intermittent faults.
- Native Wi-Fi/gateway behavior and browser launching require validation on real Windows hardware.
- The Windows binary is currently unsigned.
- Session history is memory-only and one diagnosis may run at a time.

## Release checklist

- [ ] Frontend format, lint, component, production and portfolio builds pass.
- [ ] Playwright lifecycle, reconnect, cancellation, export and seven-scenario suite passes.
- [ ] Go tests, race detector and vet pass.
- [ ] Linux and Windows release-named binaries build.
- [ ] Embedded UI and all executable demo scenarios are smoke-tested.
- [ ] Static portfolio build is inspected and cannot imply native collection.
- [ ] Load stages and environmental limits are recorded as simulated-client results.
- [ ] Native Windows capabilities, live diagnosis, browser open, export and cancellation are manually validated.
- [ ] Release screenshots and recording match the current bundle.
- [ ] No credentials, private raw results or unintended generated files are included.
