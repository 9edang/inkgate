# Production readiness

What has to be true before inkgate serves production traffic. Each section lists the checks; "Today" notes what the
repository already has, so the open work is visible.

**Gate to ship:** flat memory in the soak test, zero errors under fault injection, a clean shutdown, no critical
vulnerabilities, and a successful staging canary.

## 1. Correctness

- [ ] `make test` passes, and the known unit-test failures in [known-issues.md](known-issues.md) are fixed or accepted
- [ ] `go test -race ./...` is clean
- [ ] Fuzz config parsing and request/response merging
- [ ] `inkgate check` rejects every invalid config
- [ ] Plugins (modifier, server, client) are tested against the same Go toolchain you deploy

Today: `make test` runs the unit and integration tests. `engine/cli` `Test_pluginFuncErr` still fails (missing
testdata fixtures). Plugins load only into a binary built with the same Go version, packages and glibc; see
[guides/custom-plugins.md](guides/custom-plugins.md).

## 2. Stability

- [ ] Soak test for 2+ hours at steady load: RSS and goroutines plateau (pprof before and after)
- [ ] Run under `--memory 512m` with `GOMEMLIMIT` set to about 90% of the limit: no OOM kills
- [ ] `GOMAXPROCS` matches the CPU quota
- [ ] 10k+ idle keep-alive connections: no leak, sane memory

Today: inkgate is built with Go 1.27, which sets `GOMAXPROCS` from the cgroup CPU limit by itself (since Go 1.25), so
automaxprocs is not needed. `GOMEMLIMIT` is not set by the image.

## 3. Failure behavior

Use [toxiproxy](https://github.com/Shopify/toxiproxy) between inkgate and the backends.

- [ ] Backend slow, down, resetting connections or returning 5xx: timeouts, retries and the circuit breaker act as
      configured
- [ ] One bad backend does not stall other routes
- [ ] Graceful shutdown: SIGTERM with requests in flight drops zero of them, within the ECS `stopTimeout`
- [ ] SSE and WebSocket: a slow client, a client disconnect and a backend disconnect are all cleaned up
- [x] SSE and WebSocket end cleanly on SIGTERM (1001 for sockets, a complete response for streams)
- [x] Limits on the number of streams and connections exist: `max_connections` in `router/stream`

Today: streams have a per-endpoint connection limit (M3 3.1) and drain on shutdown (M3 3.2); metrics for them are the
rest of [milestone M3](milestones/m3-streaming.md).

## 4. Security

- [ ] `govulncheck`, `gosec` and `trivy` have no critical findings
- [ ] The image is minimal, runs as non-root, with a read-only filesystem
- [x] Request size, header size and timeouts (read, write, idle) are set: defaults since this release, see [engine/transport.md](engine/transport.md#request-limits)
- [ ] Rate limiting and auth are tested with bad input
- [ ] No secrets in config or logs

Today: `make vuln` (govulncheck) and `make sec` (gosec) run clean; `trivy` is not wired in. The image runs as user
10001 on `debian:bookworm-slim`. It cannot be `scratch`: the plugin loader needs cgo and glibc. A distroless image with
glibc (`gcr.io/distroless/base-debian12`) would work, if plugins are built against the same Debian release.

## 5. Observability

- [ ] Prometheus metrics: requests, latency histogram, errors, active connections
- [ ] Structured logs with the request ID; no log flood at `ERROR` level
- [ ] Health and readiness endpoints, wired to the ECS/ALB health check
- [ ] Alerts on error rate, p99, memory and restarts

Today: metrics through `telemetry/metrics` and the OpenTelemetry Prometheus exporter
([feature/telemetry.md](feature/telemetry.md)); request IDs are UUID v7 ([overview/flow-context.md](overview/flow-context.md));
`/__health` exists. A separate readiness endpoint and the alerts do not.

## 6. Performance

Realistic load, not only the benchmark.

- [ ] Load test with the real route config and 20–50 ms of backend latency
- [ ] Mixed payload sizes: 1 KB, 10 KB, 1 MB
- [ ] Find the maximum req/s per task, then size at about 50–60% of it
- [ ] Re-run the benchmark with 5+ rounds, pinned CPUs and `GOMAXPROCS=2`

Today: [benchmark/](../benchmark/README.md) compares inkgate with KrakenD CE on tiny bodies and a backend that answers
instantly. What to improve there is in [guides/benchmarking.md](guides/benchmarking.md).

## 7. Deployment

- [ ] The config is validated in CI (`inkgate check`) before deploy
- [ ] Staging deploy on ECS with production-like limits
- [ ] Canary or rolling deploy with circuit-breaker auto-rollback
- [ ] Rollback tested, and the previous image tag kept
- [ ] At least 2 tasks across availability zones behind the ALB

Today: multi-arch images (`linux/amd64`, `linux/arm64/v8`) are published to Docker Hub as `120200/inkgate`
by `scripts/docker-release.sh`.
