# 0003: Native Windows local-path evidence

Status: accepted for milestone 1.5

## Context

DNS, TCP, and HTTPS symptoms cannot distinguish radio instability, a local gateway problem, and degradation beyond the local network. Command output such as `netsh` is localized, changes format, and commonly exposes identifiers.

## Decision

Use documented Windows APIs behind build-constrained, testable interfaces:

- Native WLAN (`WlanOpenHandle`, `WlanEnumInterfaces`, `WlanQueryInterface`) for adapter/connection state, signal-quality percentage, SSID in volatile memory, PHY family, and link rates.
- IP Helper (`GetIpForwardTable2`, with interface classification through `GetIfEntry2`) for IPv4 default routes.
- ICMP (`IcmpCreateFile`, `IcmpSendEcho`) for unprivileged gateway echo.

No command parser is used. Basic operation requests no elevation. Each ICMP call is bounded by the probe/session deadline. If ICMP is blocked or unavailable, route presence is retained as a `route_only` fact and typed unsupported result; it is not equivalent to reachability or loss. VPN/tunnel routes, multiple connected Wi-Fi adapters, and equally preferred default routes are conservative ambiguities.

Raw SSID never crosses the platform/collector boundary into persistence. The report uses `Wi-Fi network 1` and `Wi-Fi adapter 1`; MAC/BSSID, GUID, description, and gateway address are omitted. Signal quality remains the API's 0–100 percentage and is never fabricated as dBm. Channel is unsupported because the selected connection query does not defensibly expose it without broader BSS enumeration.

Non-Windows builds return typed unsupported results. In particular, WSL cannot claim physical Windows Wi-Fi evidence.

## Consequences

Native Windows can compare radio state, local gateway behavior, and independent public destinations without administrator privileges. ICMP policy, gateway rate-limiting, route metric interpretation, adapter classification, and WLAN structure behavior still require validation on real Windows versions and hardware. No result attributes a fault to an ISP, and traceroute silence is not evidence.

Windows device-resource sampling is explicitly unsupported until defensible native CPU-pressure and available-memory semantics are selected; the collector emits one typed unsupported availability result. Gateway p95 below the configured slow threshold is healthy, and MAD/Hampel outliers alone are descriptive advanced measurements. Loss, consecutive failures, sustained severe median latency, or corroborating degraded application paths are required for stronger gateway attribution.
