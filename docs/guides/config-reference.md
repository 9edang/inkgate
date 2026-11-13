# Config reference — every `extra_config` namespace

A namespace can be written two ways in `inkgate.json`: as the **full name** (the inkgate package path) or as the
**alias**. Aliases are mapped to full names at parse time (`flow/boot/main.go` → `aliases`), so code always sees
the full name. A namespace that is neither is **refused** by `inkgate run` and `inkgate check`, with the closest known name
("did you mean"), because the gateway would ignore it and the feature would be off. For a config migrated from KrakenD,
`--allow-unknown-namespaces` accepts the KrakenD namespaces inkgate does not read; they stay ignored. `check -n`
(the schema in [`schema/schema.json`](../../schema/schema.json)) also checks the options inside each namespace and
where it is written. See [engine/cli](../engine/cli.md#what-run-and-check-refuse).

`Level` says where the key goes: **S** service (top-level `extra_config`), **E** endpoint, **B** backend.

| Alias | Full namespace | Level | Stage | Doc |
|---|---|---|---|---|
| `router` | `inkgate/engine/router` | S | entry | [engine/router](../engine/router.md) |
| `context` | `inkgate/context` | S | entry, backend | [overview/flow-context](../overview/flow-context.md) |
| `proxy` | `inkgate/engine/proxy` | E, B | endpoint | [engine/proxy](../engine/proxy.md) |
| `router/stream` | `inkgate/engine/router/stream` | E | endpoint | [engine/router](../engine/router.md#streaming-server-sent-events-and-websocket) |
| `backend/http` | `inkgate/engine/transport/http/client` | B | backend | [engine/transport](../engine/transport.md) |
| `backend/http/client` | (same, no alias) | B | backend | [engine/transport](../engine/transport.md) |
| `backend/graphql` | `inkgate/engine/transport/http/client/graphql` | B | backend | [engine/transport](../engine/transport.md) |
| `plugin/http-server` | `inkgate/engine/transport/http/server/plugin` | S | entry | [engine/async-and-plugins](../engine/async-and-plugins.md) |
| `plugin/http-client` | `inkgate/engine/transport/http/client/plugin` | B | backend | [engine/async-and-plugins](../engine/async-and-plugins.md) |
| `plugin/req-resp-modifier` | `inkgate/engine/proxy/plugin` | E, B | endpoint, backend | [engine/async-and-plugins](../engine/async-and-plugins.md) |
| `auth/validator` | `inkgate/feature/auth/jose/validator` | E | request | [feature/auth](../feature/auth.md) |
| `auth/signer` | `inkgate/feature/auth/jose/signer` | E | request | [feature/auth](../feature/auth.md) |
| `auth/revoker` | `inkgate/feature/auth/bloomfilter` | S | boot | [feature/auth](../feature/auth.md) |
| `auth/client-credentials` | `inkgate/feature/auth/oauth2client` | B | backend | [feature/auth](../feature/auth.md) |
| `security/cors` | `inkgate/feature/security/cors` | S | entry | [feature/security](../feature/security.md) |
| `security/http` | `inkgate/feature/security/httpsecure` | S | entry | [feature/security](../feature/security.md) |
| `security/bot-detector` | `inkgate/feature/security/botdetector` | S, E | entry, request | [feature/security](../feature/security.md) |
| `validation/cel` | `inkgate/feature/validation/cel` | E, B | endpoint, backend | [feature/validation](../feature/validation.md) |
| `validation/json-schema` | `inkgate/feature/validation/jsonschema` | E | endpoint | [feature/validation](../feature/validation.md) |
| `qos/ratelimit/router` | (same, no alias) | E | request | [feature/traffic](../feature/traffic.md) |
| `qos/ratelimit/proxy` | (same, no alias) | B | backend | [feature/traffic](../feature/traffic.md) |
| `qos/circuit-breaker` | `inkgate/feature/traffic/circuitbreaker/gobreaker` | B | backend | [feature/traffic](../feature/traffic.md) |
| `qos/http-cache` | `inkgate/feature/traffic/httpcache` | B | backend | [feature/traffic](../feature/traffic.md) |
| `modifier/lua-endpoint` | `inkgate/feature/transform/lua/router` | S, E | entry, request | [feature/transform](../feature/transform.md) |
| `modifier/lua-proxy` | `inkgate/feature/transform/lua/proxy` | E | endpoint | [feature/transform](../feature/transform.md) |
| `modifier/lua-backend` | `inkgate/feature/transform/lua/proxy/backend` | B | backend | [feature/transform](../feature/transform.md) |
| `modifier/martian` | `inkgate/feature/transform/martian` | B | backend | [feature/transform](../feature/transform.md) |
| `backend/amqp/producer` | `inkgate/feature/connector/amqp/produce` | B | backend | [feature/connector](../feature/connector.md) |
| `backend/amqp/consumer` | `inkgate/feature/connector/amqp/consume` | B | backend | [feature/connector](../feature/connector.md) |
| `async/amqp` | `inkgate/feature/connector/amqp/agent` | async agent | boot | [feature/connector](../feature/connector.md) |
| `backend/pubsub/publisher` | `inkgate/feature/connector/pubsub/publisher` | B | backend | [feature/connector](../feature/connector.md) |
| `backend/pubsub/subscriber` | `inkgate/feature/connector/pubsub/subscriber` | B | backend | [feature/connector](../feature/connector.md) |
| `backend/lambda` | `inkgate/feature/connector/lambda` | B | backend | [feature/connector](../feature/connector.md) |
| `telemetry/logging` | `inkgate/feature/telemetry/gologging` | S | boot | [feature/telemetry](../feature/telemetry.md) |
| `telemetry/gelf` | `inkgate/feature/telemetry/gelf` | S | boot | [feature/telemetry](../feature/telemetry.md) |
| `telemetry/logstash` | `inkgate/feature/telemetry/logstash` | S | boot | [feature/telemetry](../feature/telemetry.md) |
| `telemetry/metrics` | `inkgate/feature/telemetry/metrics` | S | all | [feature/telemetry](../feature/telemetry.md) |
| `telemetry/influx` | `inkgate/feature/telemetry/influx` | S | boot | [feature/telemetry](../feature/telemetry.md) |
| `telemetry/opentelemetry` | (same, no alias) | S | all | [feature/telemetry](../feature/telemetry.md) |

## Minimal example

```json
{
  "version": 3,
  "port": 8080,
  "timeout": "3s",
  "extra_config": {
    "telemetry/logging": { "level": "INFO", "stdout": true },
    "security/cors": { "allow_origins": ["https://app.example.com"] }
  },
  "endpoints": [
    {
      "endpoint": "/users/{id}",
      "method": "GET",
      "input_headers": ["Authorization"],
      "extra_config": {
        "auth/validator": { "alg": "RS256", "jwk_url": "https://idp.example.com/.well-known/jwks.json", "audience": ["api"] },
        "qos/ratelimit/router": { "max_rate": 100, "client_max_rate": 5, "strategy": "ip" }
      },
      "backend": [
        { "host": ["http://users:8000"], "url_pattern": "/users/{id}", "allow": ["id", "name", "email"], "group": "user" },
        { "host": ["http://orders:8000"], "url_pattern": "/orders?user={id}", "is_collection": true, "group": "orders",
          "extra_config": { "qos/circuit-breaker": { "interval": 60, "timeout": 10, "max_errors": 5 } } }
      ]
    }
  ]
}
```

What happens: CORS and logging are on for the service. `GET /users/42` needs a valid RS256 JWT for audience `api`, and
it's limited to 100 rps in total and 5 rps per IP. inkgate calls both backends in parallel and returns
`{"user": {...}, "orders": {"collection": [...]}}`. The orders backend is skipped while its circuit is open.
