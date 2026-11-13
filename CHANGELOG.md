# Changelog

All notable changes to inkgate are written here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the versions follow
[Semantic Versioning](https://semver.org/). Docker image tags use the same version
(see `scripts/docker-release.sh`).

## [Unreleased]

### Security
- Go 1.27.4 is the minimum toolchain (`go.mod`): 1.27.3's `net/http` can crash an HTTP/1 server through a race
  between `MaxBytesReader` and the headers (GO-2026-6859), and inkgate wraps every request body in `MaxBytesReader`
  (`max_body_bytes`, on by default). The Docker image already built with 1.27.4.

### Changed (breaking for plugins)
- The Docker image is built and runs on Debian 13 trixie (`golang:1.27-trixie`, `debian:trixie-slim`) instead of
  bookworm: glibc 2.41 instead of 2.36. Go plugins (`.so`) built for the bookworm image do not load; rebuild them in
  `golang:1.27-trixie`.

### Added
- `/__ready`, a readiness endpoint apart from `/__health`: 200 while the instance takes requests, 503 (with
  `Connection: close`) as soon as the shutdown starts, so a load balancer takes the instance out of rotation before its
  connections drain. It does not look at the backends. Before the server listens a probe gets "connection refused".
  `/__ready` is reserved: an endpoint cannot use it. It is skipped by OpenTelemetry like `/__health`.

### Removed (breaking for configs)
- The anonymous usage reporter (`feature/telemetry/usage`) and the `INKGATE_USAGE_URL` variable. It was off unless the
  variable was set, so no running setup changes. The `go-contrib/uuid` and `hashcash` dependencies left with it.
- OpenCensus: the `telemetry/opencensus` service namespace and all its exporters (Jaeger, Zipkin, Datadog, Stackdriver,
  X-Ray, InfluxDB, Prometheus, OC agent, logger), the OpenCensus request, pipeline and backend spans, and
  `feature/telemetry/opencensus`. OpenTelemetry (`telemetry/opentelemetry`) is the only tracing path. A config that still
  has `telemetry/opencensus` is now refused by `inkgate run` and `inkgate check` as an unknown namespace (see the
  strict config entry; `--allow-unknown-namespaces` ignores it, and the traces and metrics it exported stop). Move the exporters to `telemetry/opentelemetry` (OTLP or Prometheus). `inkgate
  audit` still reads the namespace in a KrakenD config and flags it as deprecated. Ten direct dependencies left `go.mod`
  (the OpenCensus exporters and library, `zipkin-go`), 26 lines with the indirect ones.

### Changed (breaking for configs)
- A socket endpoint without `router/stream` `allowed_origins` allows only the same origin now: an upgrade whose `Origin`
  host is not the `Host` of the request gets 403 before the backend is dialed. Before, every origin was allowed, which
  left cross-site WebSocket hijacking open by default. A request without `Origin` (not a browser) still passes. A page
  on another host needs `allowed_origins`; `["*"]` restores the old behaviour. Behind a proxy that rewrites `Host`,
  list the public origin.
- `inkgate run` and `inkgate check` refuse an `extra_config` namespace that inkgate does not read, at the service,
  endpoint, backend and async agent level: a typo (`qos/ratelimt/router`) or a KrakenD namespace. The message names the
  endpoint and the closest known namespace. Before, the key was ignored and its feature was off without a message.
  `--allow-unknown-namespaces` keeps the old behaviour, for a config migrated from KrakenD. A config that
  carries only namespaces of the docs (`docs/guides/config-reference.md`) is not affected.
- `router/stream` refuses a value that is not an object (`"router/stream": true` used to turn event mode on) and an
  option that does not exist (`max_conections`). `inkgate check` validates the section too; before, only `check -t`
  and `run` found a bad one.
- Logging: `telemetry/logging` runs on `log/slog` instead of `op/go-logging` (unmaintained since 2016), behind the same
  `logging.Logger` interface. Output is unchanged for the default and logstash formats: the same lines, colors,
  prefix, arguments joined with a space, checked against the old build. A `custom_format` may use `time`, `level`,
  `module`, `message`, `id`, `pid`, `program` and `color`; the call-site variables (`%{shortfile}`, `%{longfile}`,
  `%{shortfunc}`, `%{longfunc}`, `%{shortpkg}`, `%{longpkg}`, `%{callpath}`) are refused at startup, where they used to
  print the logger's own location and not the caller's. A bad `custom_format` is now an error and not a panic.
  `%{id}` counts per logger, not per process. `op/go-logging` left `go.mod`.
- JWT: go-jose v3 to v4 (`feature/auth/jose`, `internal/auth0`). v4 refuses an HMAC key shorter than the hash output
  (RFC 7518 3.2): 32 bytes for `HS256`, 48 for `HS384`, 64 for `HS512`. A JWK or secret that is shorter now fails on
  every token (validator) or every signature (signer); use a longer key. The `audience` rule is unchanged: every
  configured audience must be in the token (v4's own check accepts any one, so inkgate checks each). Tokens are parsed
  against the 13 supported algorithms only, and the configured `alg` is still enforced after parsing.

### Added
- `schema/schema.json` is a real JSON schema now (it was `{}`), so `inkgate check -n` works offline: it refuses unknown
  keys at the top level, in endpoints and backends and inside every namespace, a namespace at a level that does not read
  it, wrong types, bad durations and bad enum values.
- `router/stream` `max_messages_per_second` and `message_burst` for socket endpoints: a token bucket per connection on
  what the client sends (pings included); over it both sides get close 1008 (policy violation). The endpoint rate
  limit counts upgrades, not messages.
- `router/stream` `allowed_origins` and `subprotocols` for socket endpoints: an upgrade from another `Origin` gets 403,
  one offering only other subprotocols gets 400, both before the backend is dialed; only the allowed subprotocols
  reach the backend. Browsers do not apply CORS to WebSocket, so this is what stops cross-site WebSocket hijacking at
  the gateway.
- `router/stream` `keep_alive`: an SSE stream silent for that long gets a `: keep-alive` comment, so proxies in front do
  not close it. Only for `text/event-stream`, only between events.
- Stream and socket metrics under `router.stream.<endpoint>.*`: open, opened, refused, duration, messages and bytes
  each way, close codes.
- `router/stream` `max_connections`: a cap on the streams or sockets open on an endpoint at once; over it, `503` with
  `Retry-After` before the backend is called.
- JSON pass-through, opt-in with `"router": {"json_passthrough": true}`: on endpoints where nothing uses the decoded
  body, the backend's JSON is checked and sent as it is instead of decoded and encoded again (5.4× on a 100 KB body,
  half the memory). Same status, headers and JSON value; the text is the backend's (key order, number formatting).
- `benchmark/`: inkgate against KrakenD CE 2.13.11 in containers, with plugin scenarios (`make bench`).
- Lua: `resp:data(key, value)` sets one field of the response body, like `resp:data():set(key, value)`.

### Fixed
- A WebSocket backend that answers with a subprotocol the client did not offer is refused (500) instead of being
  passed on to the client, which must fail the connection on it.

### Changed
- On SIGTERM open streams end cleanly instead of being cut: WebSocket clients get a close frame 1001 (going away), SSE
  streams finish as a complete response, and the shutdown waits for both within `max_shutdown_wait_time`.
- **Breaking:** the server has request limits by default: `read_header_timeout` 10s, `read_timeout` 30s,
  `idle_timeout` 120s and the new `max_body_bytes` 10 MB (a larger body gets 413). A config that relies on no limit
  sets a negative value (`"read_timeout": "-1s"`, `"max_body_bytes": -1`). Streams and WebSocket connections are not
  affected. The config hash changes with them.
- Fixed sleeps and random ports in the HTTP server tests are replaced by waiting for the port and asking the OS for
  a free one; `make test` is green.
- Request IDs are UUID v7 (time ordered). `trust_request_id` now defaults to `false` (an incoming `X-Request-Id`
  is ignored unless enabled) and `propagate_request_id` to `true` (the ID is sent to the backends).
- Config files are parsed with sonic: parse errors read `invalid JSON: ...` with the offset, row and column.
- Decoded backend responses carry the backend status in `Metadata.StatusCode`; a merged response keeps it when all
  backends agree. An `auth/signer` endpoint now sends the backend status (a backend 201 is no longer turned into 200).
- Built with Go 1.27.3.
- The Lambda connector (`backend/lambda`) uses the AWS SDK for Go v2 (`aws-sdk-go-v2/service/lambda`) instead of the
  archived v1, which left `go.mod` with `jmespath/go-jmespath`. Same config, same credential chain. Two differences:
  `max_retries` now works (the v1 code read it as an int, but a JSON number is a float64, so it was always ignored)
  and counts retries after the first call; `endpoint` and `max_retries` also apply when `region` is not set. A config
  that sets `max_retries` now retries as many times as it says instead of the SDK default. Upgrading the SDK moved the
  indirect AWS modules (`sts`, `sso`, `smithy-go`, ...) used by the Pub/Sub connector to newer versions.
- Benchmark harness: 7 rounds and a 30 s warm-up by default (were 3 and 10 s). The report gives the median and the
  range of each gateway and marks a lead as "within noise" when the ranges overlap. New `routes` scenario: 500
  endpoints, static and parameterised, with the load spread over all of them. `meta.json` and the report record the Go
  version KrakenD CE was built with next to inkgate's. The first 7-round run shows inkgate behind KrakenD CE with 500
  routes (0.92x) and ahead in every other scenario.

### Performance
- Lua endpoint, proxy and backend scripts run on reused, isolated Lua states with code compiled once: about 13× less work per
  request, 3.3× the throughput end to end. `response.load()` in a `pre` script now fails with a clear error, and code
  that cannot be compiled is reported at startup.
- The per-request `[FLOW]` log line is no longer built below DEBUG (half the cost of the entry wrapper), and the
  logstash logger no longer formats records for levels it drops. Loggers may implement `Enabled(level int) bool`
  (`logging.LevelChecker`); `logging.Enabled` asks a logger before building a message.
- Metrics cost about 2% of throughput instead of 30%: counters and histograms are resolved once instead of looked up
  (and an 8 KB sample allocated) on every call, and no goroutine is started per backend call.

### Fixed
- With CORS `debug` on, every CORS middleware leaked a goroutine and its log lines could arrive after the request they
  describe; they are now written synchronously.
- JSON-schema validation errors listed their causes in a random order; the same body now always gets the same message.
- Data races in the metrics collector: the `/__stats` snapshot was replaced while being read.
- `client_max_rate` without `client_capacity` allowed a burst of 1 per client instead of the client rate.
- CEL `resp_metadata_status` was always 0 for decoded backends.
- Lua `resp:data(key, value)` was silently ignored.
- A CORS preflight listing allowed headers in another spelling or order than browsers use (`Origin, Content-Type`)
  was refused.
- PubSub logged two false errors for every backend that does not use it, and stayed silent on a broken PubSub config.
- An error response with an empty encoding was sent with an empty `Content-Type`.
- Numeric settings (`options_success_status`, `router/stream` `max_message_size`, audit settings) were ignored or
  rejected when the config was decoded with `internal/json`.

## [0.1.0-beta.2] - 2026-11-04

### Added
- Multi-architecture Docker image for `linux/amd64` and `linux/arm64/v8`, and release binaries for both.
  The image is cross-compiled with cgo (no emulation needed) and still ships glibc 2.36, so plugins built on
  Debian bookworm load on both.

### Changed
- The runtime image no longer runs commands at build time (non-root user is set by numeric id), so it can be
  assembled for another CPU.
- `scripts/docker-release.sh` builds and pushes a manifest list for `PLATFORMS`
  (default `linux/amd64,linux/arm64/v8`) with podman or docker buildx.

## [0.1.0-beta.1] - 2026-11-04

First public pre-release. It is a beta: configuration keys and behavior can still change before 0.1.0.

### Added
- The gateway on the gorilla/mux router: security and traffic rules, calls to one or more backends, merged
  responses.
- Server-Sent Events and WebSocket proxying.
- Chainable `plugin/http-client` plugins, and the `plugin/http-server` and `plugin/req-resp-modifier` plugin types.
- Commands `run`, `check`, `check-plugin`, `test-plugin`, `audit` and `version`.
- `Dockerfile` and `.dockerignore`: multi-stage build on Debian, non-root user, plugin folder `/opt/inkgate/plugins`.
- `scripts/docker-release.sh`: computes the next version from the git tags and builds or pushes the image.
  Releases get `X.Y.Z`, `X.Y`, `X` and `latest`; pre-releases get their own tag and `beta` or `rc`, never `latest`.
- `make lint`, `make sec`, `make vuln`, `make check-all` and `make tools`, with reports in `bin/reports`
  and CPU and memory limits for the scans; `.golangci.yml` configuration.
- [Custom plugins guide](docs/guides/custom-plugins.md): examples, build and CLI commands, and the flow with
  several plugins on one request.

### Known issues
- `make lint`, `make sec` and `make vuln` still report findings, including vulnerabilities in dependencies
  that the code calls. See `bin/reports` after running `make check-all`.

[Unreleased]: https://github.com/9edang/inkgate/compare/v0.1.0-beta.2...HEAD
[0.1.0-beta.2]: https://github.com/9edang/inkgate/compare/v0.1.0-beta.1...v0.1.0-beta.2
[0.1.0-beta.1]: https://github.com/9edang/inkgate/releases/tag/v0.1.0-beta.1
