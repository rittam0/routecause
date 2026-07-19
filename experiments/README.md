# Experiments

Real-world thresholds and claims are not established by the implementation. Record controlled runs here only after the experimental owner approves the scenario.

For each experiment, create a dated directory or document containing:

- hypothesis and supported diagnosis under evaluation;
- hardware, OS, WSL/native context, and RouteCause revision;
- configuration and threshold values;
- controlled impairment and an unimpaired control;
- expected raw signals before running;
- session JSON files, with destinations reviewed for privacy;
- observed false positives, false negatives, and unknown results;
- whether the result supports changing a threshold or rule.

Do not rewrite failed or ambiguous runs as successes. Keep benchmark and résumé claims separate until the owner reviews repeatability and sample size.

## First human WSL baseline

`results/baseline-wsl.json` is the private/local raw first baseline and should not be added to Git tracking. It exposed three milestone-one design issues: a deadline-return race allowed a 3000.458 ms DNS operation to be recorded successful at a 3-second timeout; median-only evidence concealed isolated DNS timeouts/tail latency; and once-per-second public homepage requests mixed endpoint cost with path behavior.

The long lookup and the timeout at the session boundary were different attempts. A Go `LookupHost` duration covers the whole resolver operation and may include internal retries, so it cannot be interpreted as one DNS packet RTT. Corrected experiments must record collector schedules, endpoint roles, reliability and robust-tail summaries, omitted small-sample percentiles, and whether isolated instability changed or merely qualified the bounded conclusion.

Future 30-second baselines should use the corrected defaults, retain raw output outside Git unless deliberately privacy-reviewed, and compare repeated controls before proposing threshold changes. Public health endpoints may still rate-limit or change.

The third corrected WSL baseline exposed a separate HTTPS semantic issue: six Cloudflare `HEAD` exchanges returned 404 but were presented as successes alongside six expected Google 204 responses. The timings were valid transport evidence, but 404 was not a healthy endpoint response. Schema 1.2 therefore records transport and endpoint success separately, uses bounded `GET` for the secondary default, and validates configurable expected statuses.
