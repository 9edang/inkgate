# M3: Harden live streams and WebSocket

`router/stream` (`mode: event` for SSE, `mode: socket` for WebSocket) works and is tested: 200 SSE streams outlive the
endpoint timeout with a p99 delay of 0.9 ms, and the WebSocket relay forwards ping, pong and close codes. What it lacks
is what keeps long-lived connections safe in production.

| # | Item | Size |
|---|---|---|
| 3.1 | A limit on open streams and sockets per endpoint (**done**) | S |
| 3.2 | Drain on shutdown (**done**) | M |
| 3.3 | Metrics for open connections and messages (**done** for telemetry/metrics; OpenTelemetry open) | S |
| 3.4 | SSE keep-alive comments (**done**) | S |
| 3.5 | Origin and subprotocol allow-lists at the gateway (**done**) | S |
| 3.6 | Message rate limit per WebSocket connection (**done**) | M |
| 3.7 | permessage-deflate | M |
| 3.8 | Fan-out: one backend stream to many clients | L |

## 3.1 Connection limits

Each open stream holds a goroutine, two connections and buffers, with no upper bound. Add `max_connections` to the
`router/stream` section; over it, an SSE request gets 503 and an upgrade gets 503 before the backend is called.
**Done when**: a test opens the limit plus one and the extra one is refused while the others keep working.

**Result** (November 2026): `max_connections` in `router/stream`, for both modes. The handler holds a slot for the
whole life of the stream or socket and frees it when the stream ends; a request over the limit gets 503 with
`Retry-After: 1` before the backend is dialed. Tests (`engine/router/stream_limit_test.go`) open the limit plus one for
SSE and WebSocket: the extra one is refused, the open ones keep receiving, and a slot is free again once one ends. A
negative, fractional or non-numeric value stops the gateway at startup.

## 3.2 Drain on shutdown

Hijacked WebSocket connections are not tracked by `http.Server.Shutdown`, so they drop when the process exits, and
open SSE streams hold the shutdown until `max_shutdown_wait_time`. On SIGTERM, send WebSocket clients a close frame
1001 (going away) and end SSE streams cleanly, then wait for them within `max_shutdown_wait_time`.
**Done when**: a test sends SIGTERM with open sockets and every client receives 1001.

**Result** (November 2026): the server puts a drain (`engine/transport/http/server/drain.go`) in every request context.
On shutdown it is started first: a WebSocket relay sends 1001 "server shutting down" to the client and the backend,
an SSE stream cancels its backend request so the copy ends and the client gets a complete response. Every open stream
is tracked, so after `http.Server.Shutdown` the server also waits for the hijacked sockets, which `Shutdown` does not.
Tests (`engine/router/drain_test.go`) run `RunServer` and stop it: 3 sockets each get 1001, and an SSE stream of 20 s
ends cleanly at once; before, the sockets got no close frame and the stream held the shutdown until
`max_shutdown_wait_time`. With the binary and a real SIGTERM, two clients got `1006 unexpected EOF` before and
`1001 server shutting down` after.

## 3.3 Metrics

Open streams and sockets per endpoint (a gauge), messages and bytes each way, connection length, close codes. Through
the existing metrics and OpenTelemetry layers.

**Result** (November 2026): the engine reports to a `router.StreamObserver` (opened, refused, closed with duration and
close code, every message or chunk with its size); `feature/telemetry/metrics` implements it under
`router.stream.<endpoint>.*`, with the handles of an endpoint resolved once (no allocation per message, tested).
Checked through `/__stats` with the binary: `open = 2` with two sockets connected. Still to do: an OpenTelemetry
implementation of the same interface.

## 3.4 SSE keep-alive

Proxies and load balancers in front often close a connection that stays silent for 60 s. Optional
`keep_alive: "15s"`: when the backend has sent nothing for that long, the gateway writes a `: keep-alive` comment line,
which every SSE client ignores.

**Result** (November 2026): `keep_alive` in `router/stream`. Two rules beyond the plan, each with a test: a comment is
written only for `text/event-stream` (on NDJSON or another chunked stream it would be data), and only between events
(a comment inside a paused event would break it; the end of an event is found also when its blank line is split
across chunks). The backend is read on a separate goroutine so the silence can be timed; all writes to the client stay
on one goroutine, and a client that leaves still ends the backend stream.

## 3.5 Origin and subprotocol allow-lists

The gateway accepted any `Origin` on the client side, left the check to the backend and passed any
subprotocol the client offers. Add `allowed_origins` and `subprotocols` to the socket mode, checked before the backend
is dialed.

**Result** (November 2026): both options in `router/stream`, socket mode only (on an `event` endpoint they stop the
gateway at startup: an SSE request is a fetch, which CORS already covers). A refused upgrade gets 403 (origin) or 400
(no allowed subprotocol) before a connection slot is taken or the backend is dialed. Origins match on scheme, host and
port, with `https://*.example.com` for subdomains; a request without `Origin` passes, an empty list refuses every
browser. Without the list only the same origin passes (the default was allow-all until it was changed, see the
changelog). Found on the way: gorilla's dialer takes any subprotocol the backend answers with, so a backend could hand the
client one it never offered; the gateway now refuses that, with or without a list. Tests in
`engine/router/socket_policy_test.go`, each checked to fail with its check removed.

## 3.6 Message rate limit

The endpoint rate limit counts upgrades, not messages: one connection can send without limit. A token bucket per
connection (`max_messages_per_second`, `burst`), closing with 1008 (policy violation) when exceeded.

**Result** (November 2026): `max_messages_per_second` and `message_burst` (named so, not `burst`, next to the other
`router/stream` options) for socket endpoints, with `golang.org/x/time/rate`. Choices beyond the plan: pings count (a
ping flood costs the backend like a message flood), backend messages are not limited, the bucket starts full (a
client may subscribe and authenticate at once), and the burst defaults to one second of messages. Over the limit both
sides get 1008 with the reason `message rate exceeded` and the client's further messages are dropped; it shows in the
existing `close.1008` metric. Tests (`engine/router/socket_rate_test.go`): the message over the burst never reaches
the backend, a ping flood is closed, a client within the rate is never cut; each fails with the check removed.

## 3.7 permessage-deflate

Messages cross the gateway uncompressed. Negotiate compression with the client and the backend independently
(gorilla supports it), off by default because it costs CPU. Measure the CPU cost on `websocket-echo` before enabling it
anywhere.

## 3.8 Fan-out (idea to discuss)

Many clients subscribing to the same event stream each open their own backend stream today. A fan-out mode could open
one backend stream per distinct URL and copy each event to every client on it, with a per-client buffer and a policy for
slow clients (drop or disconnect). This is a new component and changes the backend's view of its clients
(`Last-Event-ID`, authentication per client), so it needs a design note first.
