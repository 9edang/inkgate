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
  affected. The config hash (the anonymous cluster ID of the usage reporter) changes with them.
- Fixed sleeps and random ports in the HTTP server tests are replaced by waiting for the port and asking the OS for
  a free one; `make test` is green.
- Request IDs are UUID v7 (time ordered). `trust_request_id` now defaults to `false` (an incoming `X-Request-Id`
  is ignored unless enabled) and `propagate_request_id` to `true` (the ID is sent to the backends).
- Config files are parsed with sonic: parse errors read `invalid JSON: ...` with the offset, row and column.
- Decoded backend responses carry the backend status in `Metadata.StatusCode`; a merged response keeps it when all
  backends agree. An `auth/signer` endpoint now sends the backend status (a backend 201 is no longer turned into 200).
- Built with Go 1.27.3.

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
