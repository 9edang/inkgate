# flow — when things happen

The stages of a request, in order:

| Order | Package | Stage | Entry point |
|---|---|---|---|
| 1 | [boot](boot) | process start: CLI, config, logging, telemetry, plugins, JWT revocation; builds stages 4-5, hands over to entry | `boot.Main`, `boot.Run` |
| 2 | [entry](entry) | traffic enters: listener (plugins, CORS, TLS), gorilla/mux router, service-wide middleware | `entry.Router`, `entry.NewRunServer` |
| 3 | [request](request) | one endpoint's HTTP handler: tracing, bot detector, metrics, JWT, Lua, rate limit | `request.NewHandlerFactory` |
| 4 | [endpoint](endpoint) | one endpoint's pipeline: Lua, CEL, JSON schema, shadow, then calls the backends and merges | `endpoint.NewFactory` |
| 5 | [backend](backend) | one backend call: circuit breaker, rate limit, Lua, CEL, Lambda/AMQP/Pub/Sub, HTTP client | `backend.NewFactory` |

```
client → entry → request → endpoint → backend (×N) → backend service
```

`entry` and `request` are the only stages that touch gorilla/mux. Each stage lists its steps in a `steps` slice
**in request order**, numbered in comments. Full walkthrough: [../docs/flow](../docs/flow).
