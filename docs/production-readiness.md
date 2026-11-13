# Production readiness

For a team putting inkgate in front of its own services in production, on any platform (Kubernetes, ECS, Nomad, VMs
with systemd). Part 1 is what inkgate itself provides and what it does not yet; part 2 is what you have to do and
check before you send it real traffic.

## 1. What inkgate provides

| Area | Status |
|---|---|
| Correctness | `make test` (unit and integration suites) and `go test -race ./...` pass. `inkgate check` validates a config without starting |
| Request limits | on by default: `read_header_timeout` 10s, `read_timeout` 30s, `idle_timeout` 120s, `max_body_bytes` 10 MB (413 above). See [engine/transport.md](engine/transport.md#request-limits) |
| Graceful shutdown | on SIGTERM, in-flight requests finish within `max_shutdown_wait_time`; SSE streams end as complete responses, WebSockets get a close frame 1001 (going away) |
| Streams | per-endpoint `max_connections`, `keep_alive` for SSE; for sockets `allowed_origins`, `subprotocols` and a message rate per connection; metrics under `router.stream.<endpoint>.*`. See [engine/router.md](engine/router.md) |
| Observability | metrics (`telemetry/metrics`, served on its own port at `/__stats`) and OpenTelemetry with a Prometheus exporter; structured logs; a UUID v7 request ID on every request ([overview/flow-context.md](overview/flow-context.md)); a liveness endpoint, `/__health`, and a readiness endpoint, `/__ready` |
| Security | the image runs as user 10001 on `debian:trixie-slim`; `govulncheck` and `gosec` report no findings |
| Images | multi-arch (`linux/amd64`, `linux/arm64/v8`) on Docker Hub as `120200/inkgate` |

Not there yet, so plan around it:

- **Rate limits are per instance**: N replicas allow N times the configured rate ([known-issues.md](known-issues.md)).
- **`GOMEMLIMIT` is not set by the image** (see 2.2). `GOMAXPROCS` needs nothing: Go 1.25 and later read the container's
  CPU limit.
- **Plugins** (`.so`) load only into a binary built with the same Go version, packages and glibc (Go 1.27, glibc 2.41):
  build them in `golang:1.27-trixie` ([guides/custom-plugins.md](guides/custom-plugins.md)).

## 2. What you must do before going live

**Ready when:** memory stays flat in the soak test, the failure tests below give no unexpected errors, a rolling
restart drops no request, and a staged rollout to production succeeded.

### 2.1 Configuration

- [ ] Every config change is validated with `inkgate check -n` before it is deployed: it refuses an unknown namespace,
      a misspelled option and a wrong type, which the gateway would otherwise ignore
- [ ] The request limits fit your traffic: raise `max_body_bytes` and `read_timeout` together for large uploads, set
      `max_connections` on streaming endpoints
- [ ] `max_shutdown_wait_time` is shorter than the platform's termination grace period (Kubernetes
      `terminationGracePeriodSeconds`, ECS `stopTimeout`, systemd `TimeoutStopSec`), so the platform does not kill
      the process while it drains
- [ ] Endpoint `timeout`s are shorter than the timeouts of the load balancer in front
- [ ] Custom plugins are rebuilt for the exact inkgate version you deploy and checked with `inkgate test-plugin`
- [ ] No secrets in the config file or in the logs: pass them through the environment or a secret store

### 2.2 Capacity and stability

- [ ] A load test with your real routes and backend latencies (tens of milliseconds, not zero) and your payload sizes
- [ ] The highest request rate one instance sustains is known; production runs at about 50–60% of it
- [ ] A soak test of 2 hours or more at steady load: memory and goroutines level off
- [ ] `GOMEMLIMIT` is set to about 90% of the container's memory limit, and the instance is not OOM-killed under load
- [ ] Long-lived connections (keep-alive clients, SSE, WebSocket) at your expected count do not grow memory without
      bound

### 2.3 Failure behaviour

Put a fault injector such as [toxiproxy](https://github.com/Shopify/toxiproxy) between inkgate and the backends.

- [ ] A backend that is slow, down, resets connections or answers 5xx: timeouts, retries and the circuit breaker act
      as configured
- [ ] One failing backend does not slow down the routes that do not use it
- [ ] A rolling restart under load drops no request; streams and sockets reconnect

### 2.4 Security

- [ ] Your image scanner (Trivy, Grype or your registry's) reports no critical finding on the image you deploy
- [ ] The container runs read-only, as non-root (the image's default), without extra capabilities
- [ ] Rate limits and authentication are tested with bad input, and sized for N replicas (limits are per instance);
      socket endpoints have `max_messages_per_second`, since the endpoint rate limit counts upgrades, not messages
- [ ] TLS terminates where you intend: at the load balancer, or in inkgate with `tls` in the config
- [ ] Every WebSocket endpoint that a page on another host must reach has `allowed_origins` (without it only the same
      origin connects), and none uses `["*"]` for browsers with cookies: CORS does not apply to WebSocket ([engine/router.md](engine/router.md#mode-socket-websocket))

### 2.5 Observability

- [ ] Metrics are scraped: request rate, latency histogram, errors and, for streaming endpoints, open connections
- [ ] Logs are collected with the request ID, at `INFO` or above (DEBUG logs every request)
- [ ] The platform's liveness check points at `/__health` (or the `health_path` you set in the `router` section), and its
      readiness check or load balancer health check at `/__ready` ([engine/gateway.md](engine/gateway.md#readiness-for-a-load-balancer-or-kubernetes)):
      it turns 503 when the shutdown starts, so the instance leaves the rotation before it drains
- [ ] Alerts on error rate, p99 latency, memory and restarts

### 2.6 Rollout

- [ ] A staging environment with production-like limits and config
- [ ] Rolling or canary deploys that roll back automatically on errors
- [ ] Rollback tested, and the previous image tag kept
- [ ] At least two instances, in separate failure zones, behind the load balancer
