# feature — what inkgate can do

One folder per business domain. A feature that needs HTTP context has a small gorilla/mux adapter next to its
logic, in `feature/<domain>/<name>/mux`.

| Domain | Packages | Used in stage | mux adapters |
|---|---|---|---|
| `auth` | jose (JWT validate/sign), bloomfilter (revocation), oauth2client | boot, request, backend | `auth/jose/mux` |
| `security` | cors, httpsecure, botdetector | entry, request | `security/cors/mux`, `security/httpsecure/mux`, `security/botdetector/mux` |
| `validation` | cel, jsonschema | boot, endpoint, backend | — |
| `traffic` | ratelimit, circuitbreaker, httpcache | request, backend | `traffic/ratelimit/mux` (endpoint limit) |
| `transform` | lua, martian, xml, rss | boot, entry, request, endpoint, backend | `transform/lua/mux`, `transform/xml/mux` |
| `connector` | amqp, pubsub, lambda | boot (async agents), backend | — |
| `telemetry` | metrics, influx, opencensus, otel, gologging, gelf, logstash, usage, audit | every stage | `telemetry/metrics/mux`, `telemetry/opencensus/mux`, `telemetry/otel/mux` |

Each package reads its own `extra_config` namespace and does nothing when that namespace is absent.
