# 0002: Robust sampling, schedules, and diagnostic endpoints

Status: accepted for milestone one correction

## Context

The first human WSL baseline produced a successful 3000.458 ms DNS lookup at a nominal 3-second deadline, separate DNS timeouts, and a slow public homepage despite healthy TCP medians. Median-only diagnosis hid isolated instability, while the mean overreacted to one extreme. Probing every collector every second also imposed unnecessary HTTPS work.

## Decision

Treat a nil-returning operation as timeout if its context is already done; otherwise preserve a near-deadline success as a tail event. Summaries expose reliability rates, failure runs, median, MAD/Hampel outliers, descriptive mean/range, p95 from 20 samples, and p99 from 100. Diagnosis distinguishes typical, tail, sustained, and isolated signals and remains unknown for severe or conflicting unexplained signals.

Schedule device, DNS/TCP, and HTTPS at 1s, 2s, and 10s defaults respectively, with staggered starts. Prefer small configurable HTTPS health responses from independent operators over public homepages.

## Consequences

A quiet session can retain the exact bounded no-material-problem conclusion while acknowledging isolated instability. Small samples correctly omit percentiles, and a 30-second HTTPS series normally supports only typical/reliability evidence. Public endpoint behavior and the statistical thresholds remain experimental assumptions.
