# M4: Traffic and resilience features

New capabilities that teams usually expect from a gateway next. None of them exists today; each needs a short design
note (config shape, where it sits in [../flow](../flow), its interaction with the features around it) before code.

| # | Feature | What it gives | Size |
|---|---|---|---|
| 4.1 | Request coalescing | identical concurrent GETs share one backend call | M |
| 4.2 | Retries with backoff | a failed idempotent call is tried again | M |
| 4.3 | Shared rate limit (Redis) | one limit across all instances | L |
| 4.4 | Endpoint response cache | cache the final, merged answer | M |
| 4.5 | Backend health and outlier detection | stop sending traffic to a failing host | L |
| 4.6 | HTTP/2 and h2c to backends | multiplexed connections, gRPC-friendly | M |

## 4.1 Request coalescing

When N clients ask for the same thing at the same moment, the backend gets N calls. A `singleflight` keyed by method,
URL and the forwarded headers lets them share one call. GET and HEAD only, opt-in per backend
(`"coalesce": true`). Measured win: run `http-cache-hit` style traffic against a slow backend and count backend hits.

## 4.2 Retries with backoff

Today a failed call fails the request (or returns a partial merge). Retries for idempotent methods, on connection
errors and chosen status codes, with exponential backoff and jitter, bounded by the endpoint timeout. They must count
as one call for the circuit breaker's error count, or the breaker opens too late.

## 4.3 Shared rate limit

Rate-limit buckets are per instance (in [../known-issues.md](../known-issues.md)): N instances allow N times the rate.
A Redis-backed limiter (GCRA in one round trip, with a local fallback when Redis is down) gives one limit for the
fleet.

## 4.4 Endpoint response cache

`qos/http-cache` caches each backend's answer, but the merged answer is built again on every request: with 4
backends that is the merge cost measured in M2 item 2.7, every time. An endpoint-level cache keyed by the request,
honoring `cache_ttl`, with stale-while-revalidate, would skip the whole pipeline. The cache-hit job already shows
153% of base with backend caching only.

## 4.5 Backend health

The load balancer picks among the configured hosts with no notion of health. Passive outlier detection (eject a host
after N consecutive errors for a growing time) and optional active health checks.

## 4.6 HTTP/2 to backends

The client side speaks HTTP/1.1. HTTP/2 (TLS) and h2c (cleartext) backends would reduce connections and allow gRPC
backends through the gateway; gRPC itself (and gRPC-Web translation) would be a later step.
