# 0001: Local-first diagnostic sessions

Status: accepted for milestone one

## Context

RouteCause must produce inspectable evidence without requiring an account, hosted service, or privileged agent. Network diagnostics can expose browsing-related destinations and machine context.

## Decision

The CLI runs collectors and diagnosis locally, stores one versioned JSON session at a user-selected path with mode `0600`, and prints a concise rendering of the typed diagnosis. There is no telemetry, database, cloud service, or upload path. Raw measurements remain distinct from summaries, derived evidence, and conclusions.

## Consequences

Sessions are portable and auditable, and an offline test suite can use deterministic fake data. The user is responsible for retaining or sharing a session. Cross-session analysis and remote support workflows are deliberately absent.
