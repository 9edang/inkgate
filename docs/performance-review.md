# Performance and resource review

A code review of the request path (entry → router → proxy → merge → render) and of the metrics, rate-limit, JWT and
Lua code, done in November 2026. No profile was taken: every finding comes from reading the code. Costs are the
measured ones from the [milestone baseline](milestones/README.md#the-baseline-the-milestones-are-measured-against)
where one exists, otherwise "unmeasured".

Each finding was then checked against the code. The status column says what that check found:

| Status | Meaning |
|---|---|
| **confirmed** | the code does what the finding says (file and line given) |
| **tracked** | confirmed, and a milestone item already covers it |
| **unverified** | plausible, not checked yet; profile before changing anything |
| **wrong** | the code or the runtime does not behave that way |

## A. CPU and allocations per request

| # | Finding | Status | Where it goes |
|---|---|---|---|
| 1 | Metrics allocate a new sample on every call: `Histogram()` evaluates `defaultSample()` as an argument, so `NewUniformSample(1028)` (an 8 KB slice) is allocated per call even when the histogram exists. A goroutine is started and label strings are built per backend call. | confirmed: `feature/telemetry/metrics/proxy.go:66-72, 97`, `metrics.go:179` | [M2 2.5](milestones/m2-request-cost.md#25-cheaper-metrics-check-first): **fixed**, metrics cost 30% → 1.7% |
| 2 | Lua builds a new VM per request, registers every decorator and recompiles the sources. 28% of base. | tracked: `feature/transform/lua/proxy/proxy.go:71-89` | [M2 2.1](milestones/m2-request-cost.md#21-reuse-lua-vms): **fixed** for lua-endpoint, lua-proxy and lua-backend |
| 3 | JSON is decoded into `map[string]any` and encoded again; 100 KB costs 17× CPU and 160 MB peak. | tracked | [M2 2.2](milestones/m2-request-cost.md#22-pass-large-responses-through-untouched): **fixed**, opt-in, 5.4× |
| 4 | One goroutine per `no-op` response to close the body on cancel. | measured: 0.37 µs per response, 0.3% of a request; AfterFunc is slower | [M2 2.3](milestones/m2-request-cost.md#23-no-goroutine-per-no-op-response): **dropped**, not the cause |
| 5 | The `[FLOW]` summary line is formatted with `fmt.Sprintf` on every request, also when DEBUG is off, and copies the backend-call slice. | tracked: `flow/entry/server.go:45-46` | [M2 2.4](milestones/m2-request-cost.md#24-free-log-calls-below-the-level): **fixed** at INFO and above |
| 6 | Merge clones each backend request with `url.Parse(r.URL.String())`, plus a goroutine and two channels per call. | unverified | [M2 2.7](milestones/m2-request-cost.md#27-lighter-merge-check-first) |
| 7 | Backend request setup copies the URL, the headers and the request more than once; the flow middleware copies the header map to add the request ID. | unverified | M2 2.7 |
| 8 | `gzip.NewReader` per response, not pooled. Matters only when a backend sends `Content-Encoding: gzip` itself, since Go's transport decompresses otherwise. | unverified, likely minor | none |
| 9 | Flow context: `uuid.NewV7` per request (the library serializes it with a lock), an `RWMutex` in `Flow`, and `r.WithContext` copies the request. | unverified, likely minor | none |
| 10 | `r.URL.Query()` is parsed on every request, also for endpoints that forward no query string. | unverified | none |
| 11 | A timer per request (`WithTimeout`) and `time.AfterFunc` on the stream path. | unverified, likely minor | none |

## B. Contention and scaling

| # | Finding | Status | Where it goes |
|---|---|---|---|
| 12 | The client rate limiter uses one shard (one `RWMutex` over all clients) unless `num_shards > 1`. The cleanup may hold the write lock while it scans every key, which would add latency spikes with many clients. | confirmed for the single shard: `feature/traffic/ratelimit/router/store.go:12`; cleanup locking unverified | new: document `num_shards` and pick a default above 1 |
| 13 | Rate-limit buckets are per instance: N instances allow N × the rate. | tracked | [known-issues](known-issues.md), [M4 4.3](milestones/m4-traffic-resilience.md#43-shared-rate-limit) |
| 14 | The global token bucket of an endpoint is one mutex. | unverified | none |
| 15 | The Lua binder, the metrics registry and `log.Logger` each take a lock; at DEBUG every line takes the logger mutex. | unverified | M2 2.4, 2.5 |
| 16 | No cap on connections per backend host (`MaxConnsPerHost` not exposed), so a slow backend piles up connections and goroutines until the endpoint timeout. | unverified | new: candidate for [M4](milestones/m4-traffic-resilience.md) |

## C. Memory and resource limits

| # | Finding | Status | Where it goes |
|---|---|---|---|
| 17 | No request body size limit: there is no `http.MaxBytesReader` in the code. For multi-backend endpoints with a body, `CloneRequest` buffers the whole body once per backend. | confirmed: no `MaxBytesReader` in the repository | **fixed**: `max_body_bytes`, 10 MB by default |
| 18 | `read_timeout`, `write_timeout`, `idle_timeout` and `read_header_timeout` default to 0 (no limit), which leaves the server open to slow clients unless the config sets them. | confirmed: `engine/config/parser.go:190-193` only parses them; only `timeout` has a default (`config.go:461`) | **fixed**: defaults, see [transport.md](engine/transport.md#request-limits) |
| 19 | Large JSON bodies cost about 17× memory. | tracked | same as 3 |
| 20 | Metric names and rate-limit client keys grow with label and client cardinality. | unverified | none |

## D. Correctness

| # | Finding | Status | Where it goes |
|---|---|---|---|
| 21 | With only `client_max_rate` set, every client gets a burst of 1. | confirmed: `feature/traffic/ratelimit/router/ratelimit.go:73` tests `cfg.MaxRate` (the endpoint rate, 0 here) instead of `cfg.ClientMaxRate` | [M1 1.5](milestones/m1-confirmed-bugs.md#15-client-rate-limit-burst-is-1): **fixed** |
| 22 | An unknown `extra_config` namespace is ignored silently, so a typo turns a feature off. | tracked | [M5 5.1](milestones/m5-operability.md#51-strict-config) |

## E. Platform

| # | Finding | Status | Where it goes |
|---|---|---|---|
| 23 | sonic is fast on amd64; on arm64 (Graviton) it uses a slower fallback for some paths. | partly: the `linux/arm64/v8` image builds and is released; arm64 speed unmeasured | measure before an arm64 deployment |
| 24 | With `--cpus 2`, Go sees every host core, so `GOMAXPROCS` is too high and tail latency rises. | wrong: since Go 1.25 the runtime sets `GOMAXPROCS` from the cgroup CPU limit; inkgate uses Go 1.27 | none (set it explicitly in the benchmark only for the record) |

## Safety defaults (17 and 18)

Not a speed problem: these decide whether one bad client can take an instance down. Proposal:

| Setting | Default today | Proposed default |
|---|---|---|
| `read_header_timeout` | none | 10 s |
| `read_timeout` | none | 30 s |
| `idle_timeout` | none (falls back to `read_timeout`) | 120 s |
| `write_timeout` | none | none (it would cut long streams and SSE; rely on `timeout` per endpoint) |
| request body size | unlimited | `max_body_bytes`, 10 MB by default, checked with `http.MaxBytesReader` |

A default changes behaviour for configs that rely on no limit today, so it belongs in the CHANGELOG as a breaking
change. The checklist entry is in [production-readiness.md](production-readiness.md#4-security).

## Order of work

| Order | Work | Findings | Why |
|---|---|---|---|
| 1 | ~~Fix the client rate-limit burst~~ (done) | 21 | a wrong answer to a correct config, one line |
| 2 | ~~Safety defaults: body limit and server timeouts~~ (done) | 17, 18 | protects every deployment; small |
| 3 | ~~Metrics: one sample per histogram, handles resolved at startup, no goroutine per call~~ (done) | 1 | 20% measured cost, root cause known |
| 4 | ~~Log guard~~ (done); `context.AfterFunc` measured slower, dropped | 4, 5 | small, already planned |
| 5 | ~~Lua VM pool~~ (done, all three Lua modules) | 2 | largest measured cost, needs the isolation test of M2 2.1 |
| 6 | ~~Pass-through for unread large bodies~~ (done, opt-in) | 3 | largest memory cost |
| 7 | Profile merge and backend setup, then decide | 6, 7, 9, 10 | unverified; profile first ([M5 5.3](milestones/m5-operability.md#53-pprof)) |

Every change in steps 3 to 7 comes with a benchmark (`go test -bench -benchmem`) that shows the allocations before and
after.
