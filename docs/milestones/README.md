# Milestones

What to fix, speed up and add next, in the order it pays off. Every item comes with the evidence behind it: a
benchmark figure, a line of code or a reproduction. An item without evidence is marked as an idea to check first.

| Milestone | Theme | Size | Why first | Status (November 2026) |
|---|---|---|---|---|
| [M1](m1-confirmed-bugs.md) | Fix confirmed bugs | small | each one is reproduced and gives wrong behavior today | 7 of 8 fixed; 1.4 open |
| [M2](m2-request-cost.md) | Cut the cost of a request | medium | the slowest features cost 40% to 93% of throughput, measured | 2.1, 2.2, 2.4, 2.5 done; 2.3 dropped; 2.6 to 2.8 open |
| [M3](m3-streaming.md) | Harden live streams and WebSocket | medium | the stream code had no limits, no metrics and no shutdown drain | 3.1 to 3.6 done; 3.7, 3.8 and OpenTelemetry stream metrics open |
| [M4](m4-traffic-resilience.md) | Traffic and resilience features | large | what an API gateway is usually expected to do next | not started |
| [M5](m5-operability.md) | Config, observability and test hygiene | medium | makes everything above safer to change | 5.1, 5.2 and 5.5 done; the rest open |

M1 and the quick items of M2 can start at once; M3 to M5 can run in parallel after that.

## The baseline the milestones are measured against

From the benchmark harness (a sibling directory of this repository), final run of 4 November 2026:
inkgate on **one CPU core and 2 GB of RAM**, 128 connections, median of 3 runs. "% of base" compares with the
plainest job, forwarding a 1 KB JSON document. This is not the setup of [benchmark/](../../benchmark/README.md) (inkgate
against KrakenD CE, 2 CPUs and 512m each), so do not compare these numbers with that report; see
[guides/benchmarking.md](../guides/benchmarking.md#11-reconcile-the-docs).

| Job | Requests/s | % of base | CPU µs per request | Peak MB |
|---|--:|--:|--:|--:|
| forward 1 KB JSON (base) | 13,674 | 100% | 63.9 | 35 |
| forward 100 KB JSON | 893 | 7% | 1,084 | 160 |
| pass 16 KB through untouched (`no-op`) | 8,598 | 63% | 93.5 | 37 |
| merge 4 backends | 5,271 | 39% | 168.5 | 55 |
| JWT HS256 validation | 8,145 | 60% | 112.8 | 37 |
| Lua post-script | 3,790 | 28% | 250.5 | 80 |
| metrics on | 10,997 | 80% | 82.3 | 37 |
| logging at DEBUG | 11,103 | 81% | 79.8 | 49 |
| JSON Schema validation | 11,442 | 84% | 75.8 | 35 |
| 200 SSE streams | 19,999 events/s, p99 delay 0.9 ms | | 12.9 per event | 28 |
| 200 WebSocket connections | 60,124 round trips/s (the backend alone: 427,880) | | 15.4 per message | 39 |

The comparison with KrakenD CE is in the [README](../../README.md#benchmark) and [benchmark/](../../benchmark/README.md),
the one place that makes a performance claim.

## How an item is done

- **Measured, not guessed**: an M2 item is finished when the benchmark job it targets moves, by the amount its
  "done when" names, in `make bench S=<job>` against the same baseline. A `pprof` profile goes with every claim
  about where time goes.
- **Behavior change written down**: anything that changes a response or a default goes into the feature's page in
  [../feature](../feature) and into [../known-issues.md](../known-issues.md) when relevant.
