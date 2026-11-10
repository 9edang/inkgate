# M2: Cut the cost of a request

On one core the gateway is CPU-bound in every job, so CPU microseconds per request is throughput. The table lists the
biggest measured costs (from the [baseline](README.md#the-baseline-the-milestones-are-measured-against)), largest
first. Items marked **check first** have a likely cause in the code but no profile yet: take a `pprof` CPU profile of
the job before changing anything.

| # | Item | Today | Target | Size |
|---|---|---|---|---|
| 2.1 | Reuse Lua VMs instead of building one per request | 3,790 req/s, 250 µs | above 8,000 req/s | M |
| 2.2 | Pass large responses through without decoding when nothing reads them | 893 req/s at 100 KB, 160 MB | 3× or more | M |
| 2.3 | Remove the goroutine started for every `no-op` response | 8,598 req/s at 16 KB | above 11,000 req/s | S |
| 2.4 | Skip work for log lines that will not be printed | DEBUG costs 27% vs INFO | no cost at INFO and above | S |
| 2.5 | Make metrics cheaper per request (check first) | metrics cost 20% | under 8% | M |
| 2.6 | Cache validated JWTs (check first) | JWT costs 40% | under 15% | M |
| 2.7 | Lighter merge of several backends (check first) | 4 backends: 39% of base | 50% | M |
| 2.8 | Fewer allocations per WebSocket message | 60,124 round trips/s | 90,000 | S |

## 2.1 Reuse Lua VMs

**Evidence**: `feature/transform/lua/proxy/proxy.go:71` builds a new binder (a new `gopher-lua` state) for every
request, registers the decorators on it and loads the sources again, then closes it. A Lua post-script that adds one
field drops the endpoint to 28% of base.

**Plan**: keep a `sync.Pool` of prepared states per endpoint: decorators registered and sources compiled once (to a
`FunctionProto`), globals reset between uses. `live: true` (reload on every request) keeps today's path. Check the
router and backend Lua modules for the same pattern.

**Done when**: `lua-proxy-script` above 8,000 req/s, and a test proves that a global set by one request is not visible
to the next.

## 2.2 Pass large responses through untouched

**Evidence**: a 100 KB JSON answer costs 1,084 µs of CPU and 160 MB peak, 17× the 1 KB job, because the backend body is
decoded into `map[string]any` and then encoded again. The `no-op` path is far cheaper per byte.

**Plan**: at startup, decide per endpoint whether anything reads the decoded data: a single backend with no `allow`,
`deny`, `mapping`, `group`, `target`, `flatmap_filter`, no CEL `resp_` rule, no Lua, no response modifier, no
`output_encoding` change. If nothing does, use the `no-op` path automatically and keep the backend's status and
`Content-Type`. Log the decision at DEBUG so it can be seen.

**Done when**: `proxy-json-100k` at 3× or more with the same response bytes, and every response-changing feature
still turns the fast path off (a table-driven test per feature).

## 2.3 No goroutine per `no-op` response

**Evidence**: `engine/proxy/proxy.go` `NewReadCloserWrapper` starts a goroutine for every response
(`go wrapper.closeOnCancel()`) only to close the body when the context ends.

**Plan**: use `context.AfterFunc(ctx, rc.Close)` and stop it on EOF or `Close`. Same behavior, no goroutine.

**Done when**: `proxy-noop-raw` above 11,000 req/s and the SSE tests (which rely on the close on disconnect) pass.

## 2.4 Free log calls below the level

**Evidence**: `flow/entry/server.go:45` formats the `[FLOW]` line with `fmt.Sprintf` on every request, before
`logger.Debug` drops it at INFO or ERROR, so every request pays for a line nobody sees. At DEBUG the gateway runs at
81% of base; formatting and writing the per-request lines is the likely cost (not profiled yet).

**Plan**: give the logger a cheap `Enabled(level)` check (or pass arguments, not a formatted string) and guard the
per-request lines with it. At DEBUG, write through a buffered writer.

**Done when**: an allocation benchmark of the entry handler shows no allocation from logging at INFO, and
`telemetry-logging-debug` gains at least 10%.

## 2.5 Cheaper metrics (check first)

**Evidence**: turning metrics on costs 20% (10,997 vs 13,674 req/s). The collector uses named counters and histograms
from `rcrowley/go-metrics`. The code review ([performance-review.md](../performance-review.md) #1) found the main cause:
`ProxyMetrics.Histogram()` passes `defaultSample()` as an argument, so a new `NewUniformSample(1028)` (an 8 KB slice)
is allocated on every call, even when the histogram already exists. On top of that, `NewProxyMiddleware` starts a
goroutine and builds the label strings per backend call (`feature/telemetry/metrics/proxy.go:66-72`).

**Plan**: resolve the counters and histograms of each endpoint and backend once at startup (all four
complete/error combinations are already registered there) and keep the handles; update them inline instead of in a
goroutine. A profile before and after confirms the gain.

**Done when**: metrics cost under 8% in `telemetry-metrics`.

## 2.6 Cache validated JWTs (check first)

**Evidence**: HS256 validation costs 40% (8,145 req/s). Every request parses the token, checks the signature and the
claims.

**Plan**: after a profile, a small LRU keyed by a hash of the token, holding the verified claims until the earlier of
`exp` and a configured maximum (`cache_validated_tokens: "30s"`, off by default). The revocation check (bloom filter,
CEL `JWT` rules) still runs on every request.

**Done when**: `auth-jwt-hs256` costs under 15%, with tests that an expired token, a revoked token and a token signed
with a rotated key are all refused while cached.

## 2.7 Lighter merge (check first)

**Evidence**: four backends in parallel run at 39% of base, against an expected ~4× the work of one backend for the
calls themselves. The profile will show whether the merge (map copies per backend, the 85% timeout bookkeeping) or the
four HTTP calls dominate.

**Done when**: `aggregate-4-backends` at 50% of base, or the profile shows the cost is the four calls themselves.

## 2.8 WebSocket allocations

**Evidence**: the relay (`engine/transport/http/socket/socket.go`) copies each message through `NextReader` and
`NextWriter` with default buffers.

**Plan**: give the `Upgrader` and the `Dialer` a shared `WriteBufferPool` and explicit read and write buffer sizes,
and reuse one copy buffer per direction.

**Done when**: `websocket-echo` above 90,000 round trips/s with the same p99.
