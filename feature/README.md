# feature — what inkgate can do

One folder per business domain. A feature that needs HTTP context keeps its router code next to its logic
(for example `jose_router.go`, or the `router` / `gateway` sub-package listed below).

| Domain | Packages | Used in stage | Router code |
|---|---|---|---|
| `auth` | jose (JWT validate/sign), bloomfilter (revocation), oauth2client | boot, request, backend | `auth/jose/jose_router.go` |
| `security` | cors, httpsecure, botdetector | entry, request | `security/cors/cors_router.go`, `security/httpsecure/middleware.go`, `security/botdetector/gateway` |
| `validation` | cel, jsonschema | boot, endpoint, backend | — |
| `traffic` | ratelimit, circuitbreaker, httpcache | request, backend | `traffic/ratelimit/router` (endpoint limit) |
| `transform` | lua, martian, xml, rss | boot, entry, request, endpoint, backend | `transform/lua/router`, `transform/xml/render.go` |
| `connector` | amqp, pubsub, lambda | boot (async agents), backend | — |
| `telemetry` | metrics, influx, opencensus, otel, gologging, gelf, logstash, usage, audit | every stage | `telemetry/metrics/metrics_router.go`, `telemetry/opencensus/endpoint.go`, `telemetry/otel/otel_router.go` |

Each package reads its own `extra_config` namespace and does nothing when that namespace is absent.
