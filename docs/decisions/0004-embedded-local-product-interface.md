# 0004: Embedded localhost product interface

Status: accepted for milestone 2

## Context

The diagnostic CLI and native collectors are inspectable but require command-line knowledge. An ordinary user needs one action, honest progress, plain-language evidence, and export without introducing a hosted service or privacy boundary.

## Decision

Build a hand-written React/TypeScript interface with Vite. Commit the production bundle under `internal/webassets/dist` and embed it in the Go executable. A small Go HTTP service binds exclusively to `127.0.0.1`, opens that fixed URL in the default browser, and exposes versioned JSON and Server-Sent Event endpoints.

Maintain one active 60-second snapshot. Store at most ten privacy-safe summary records in memory and no browser state. SSE uses replayable integer event IDs so refresh/reconnect observes the existing run. The primary surface reports five factual layer cards and a bounded test summary; confidence and technical interpretation remain under Advanced report. Stable internal diagnosis codes remain for schema compatibility but are not presented as proof of cause. Demo mode is a process-level flag with deterministic observed-pattern fixtures and persistent visual/API labels; it cannot mix live and fixture measurements.

Validate Host and Origin, cap request bodies/headers and SSE clients, reject unknown JSON, expose no target configuration, and cancel the engine on API cancellation or process shutdown. Apply a defense-in-depth privacy sanitizer before session retrieval and export.

## Consequences

RouteCause remains a single local executable with no login, analytics, upload, database, cloud service, or background daemon. History disappears when the process exits. Browser opening depends on the operating system's default-browser integration. A Vite/Node toolchain is needed only when changing frontend source; users of the compiled executable do not need Node.
