# Native Windows acceptance runs

RouteCause was manually exercised on native Windows 11 on July 19, 2026. Three
controlled 60-second snapshots were exported from the release-named Windows
binary. This page records privacy-safe aggregate evidence only; the raw session
reports are deliberately excluded from the repository.

These runs validate that the product distinguishes materially different
observed network states. They do not establish population-wide accuracy,
identify an ISP, or prove ownership of an underlying fault.

## Controlled conditions

| Condition | Observed result | Selected aggregate evidence |
| --- | --- | --- |
| Wi-Fi disabled | Wi-Fi/Ethernet showed repeated failures during the test | DNS, gateway, TCP and HTTPS checks recorded no successful network operations; the gateway was unreachable throughout the snapshot |
| Wi-Fi enabled with Proton VPN active | Gateway showed repeated failures during the test | Wi-Fi remained connected; gateway ICMP recorded 0/30 replies, while both TCP destinations remained reachable on nearly all attempts and public HTTPS completed partially |
| Wi-Fi enabled without VPN | No significant problem was observed during the test | Gateway 30/30, HTTPS 12/12 and TCP 60/60 succeeded; DNS completed 89/91 checks successfully |

The VPN state was a human-controlled test condition, not a value detected or
stored by RouteCause. The second run therefore demonstrates different gateway
ICMP and public-probe behaviour while the VPN was active; it does **not** prove
that Proton VPN, the physical gateway, or any other component caused a fault.

## What was manually validated

- The native Windows executable launched without administrator privileges.
- The embedded browser interface opened and completed full snapshots.
- Native Wi-Fi, route and gateway evidence reached the interface.
- The interface produced distinct bounded summaries for disconnected,
  VPN-active and ordinary Wi-Fi conditions.
- Privacy-safe JSON export completed for all three runs.
- A normal connection was not marked degraded because of statistical latency
  outliers alone.
- Unsupported Windows device-resource collection appeared as neutral,
  one-shot unsupported evidence rather than repeated system errors.

## Remaining validation boundary

These are three controlled runs on one Windows computer and network. They are
acceptance evidence, not a capacity or accuracy guarantee. Additional adapters,
routers, VPN implementations, captive portals, Ethernet paths and Windows
versions may behave differently. RouteCause reports observations from its
measurement window and does not claim definitive root-cause attribution.
