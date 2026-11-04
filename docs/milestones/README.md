# Milestones

What to fix, speed up and add next, in the order it pays off. Every item comes with the evidence behind it: a
benchmark figure, a line of code or a reproduction. An item without evidence is marked as an idea to check first.

| Milestone | Theme | Size | Why first |
|---|---|---|---|
| [M1](m1-confirmed-bugs.md) | Fix confirmed bugs | small | each one is reproduced and gives wrong behavior today |
| [M2](m2-request-cost.md) | Cut the cost of a request | medium | the slowest features cost 40% to 93% of throughput, measured |
| [M3](m3-streaming.md) | Harden live streams and WebSocket | medium | new code: correct, but no limits, no metrics, no shutdown drain |
| [M4](m4-traffic-resilience.md) | Traffic and resilience features | large | what an API gateway is usually expected to do next |
| [M5](m5-operability.md) | Config, observability and test hygiene | medium | makes everything above safer to change |

M1 and the quick items of M2 can start at once; M3 to M5 can run in parallel after that.

## The baseline the milestones are measured against

From the benchmark harness in `../benchmark` (a sibling directory of this repository), final run of 4 November 2026:
inkgate on **one CPU core and 2 GB of RAM**, 128 connections, median of 3 runs. "% of base" compares with the
plainest job, forwarding a 1 KB JSON document.

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

Compared with KrakenD CE 2.13.11 on the same core: inkgate is 1.45× faster on average and uses 68% of its CPU per
request. inkgate-gin is level with KrakenD (0.98×). Most of the optimizations below apply to the shared code, so
they would also help inkgate-gin.

## How an item is done

- **Measured, not guessed**: an M2 item is finished when the benchmark job it targets moves, by the amount its
  "done when" names, in `make bench S=<job>` against the same baseline. A `pprof` profile goes with every claim
  about where time goes.
- **Behavior change written down**: anything that changes a response or a default goes into the feature's page in
  [../feature](../feature) and into [../known-issues.md](../known-issues.md) when relevant.
- **Both builds**: say for each item whether it also applies to inkgate-gin (the shared `engine/` and `feature/`
  code usually does; `engine/router/mux` and `engine/transport/http/socket` do not).
