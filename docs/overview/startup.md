# Startup

From `inkgate run -c inkgate.json` to serving the first request.

```mermaid
flowchart TD
    A["cmd/inkgate main()"] --> M["boot.Main(entry.Router)"]
    M --> B["signal handler: SIGINT/SIGTERM cancel ctx"]
    B --> C["register XML/RSS decoders + renders<br/>register config aliases"]
    C --> D{"FC_ENABLE set?"}
    D -- yes --> E["flexibleconfig parser<br/>(templates, settings, partials)"]
    D -- no --> F["koanf parser"]
    E --> G["engine/cli: run command parses + validates config"]
    F --> G
    G --> H["boot.Run(ctx, cfg, router)"]

    subgraph RUN ["boot.Run (shared)"]
      H1["1 logger: logstash | gologging (+GELF) | stdout"] --> H2["2 DNS TTL, load .so plugins"]
      H2 --> H3["3 router.NewMetrics → metrics + /__stats<br/>Influx, OpenCensus, OTel<br/>JWK cache, JWT revocation (bloom filter + CEL)"]
      H3 --> H4["4 backend.NewFactory → endpoint.NewFactory"]
      H4 --> H5{"async agents?"}
      H5 -- no --> H6["router.Serve(ctx, Services)"]
      H5 -- yes --> H7["errgroup: AMQP agents + router.Serve"]
    end
    H --> H1
    H6 --> V["entry.Serve: build entry + request, register endpoints, listen"]
    H7 --> V
```

## Step details

| # | What | Code | Notes |
|---|---|---|---|
| 0 | CLI + parse | `flow/boot/main.go`, `engine/cli`, `engine/config` | fails fast on invalid config; `check` stops here |
| 1 | logger | `flow/boot/logging.go` | first configured wins; the GELF writer is passed to entry for access logs |
| 2 | plugins | `flow/boot/plugins.go` | loads every `.so` matching `plugin.pattern` in `plugin.folder` |
| 3 | telemetry | `entry.Router.NewMetrics` + `flow/boot/telemetry.go` | entry creates the collector and its `/__stats` server; exporters do nothing without their namespace |
| 3b | token rejecter | `flow/boot/telemetry.go` `newTokenRejecter` | bloom filter (`auth/revoker`) + CEL rules that use `JWT` |
| 4 | shared stages | `flow/backend`, `flow/endpoint` | builders only; nothing runs yet |
| 5 | serve | `Serve` in `flow/entry/serve.go` | builds one pipeline per endpoint + method, then listens |

Pipelines are built **once at startup**, not per request. A config change needs a restart.

## Shutdown

Cancelling the context (SIGINT/SIGTERM) stops the listener gracefully (`max_shutdown_wait_time`), stops async
agents, and flushes OpenTelemetry (`closeTelemetry`).
