# Milestone 2 local HTTP/SSE load observation

Recorded 2026-07-19 in the current WSL development environment using `routecause-load` and the deterministic executable demo. These are simulated SSE clients, not users. They are a single-machine engineering observation, not a capacity promise.

| Simulated clients | Connect p50 | Connect p95 | Connect p99 | Completed streams | Streams/s | Error rate | Peak server RSS | Server CPU |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 0.769 ms | 0.891 ms | 0.891 ms | 10 | 13.78 | 0% | 10.4 MB | 0.01 s |
| 100 | 6.484 ms | 11.631 ms | 12.808 ms | 100 | 137.79 | 0% | 15.0 MB | 0.05 s |
| 1,000 | 105.667 ms | 140.761 ms | 142.717 ms | 1,000 | 1,322.58 | 0% | 44.7 MB | 0.42 s |
| 10,000 | 592.661 ms | 1,080.820 ms | 1,098.437 ms | 10,000 | 8,474.31 | 0% | 230.6 MB | 3.55 s |

The deterministic run lasts about 720 ms, so low-stage throughput mostly reflects fixture duration rather than request-processing capacity. Connect latency is measured from request start until response headers. CPU is the delta in Linux `/proc/<pid>/stat`; RSS is sampled from `/proc/<pid>/status`. Linux clock-tick conversion currently assumes 100 ticks per second, as used by the measurement environment. Reproduce with the command in the README and preserve environmental limits with the result.
