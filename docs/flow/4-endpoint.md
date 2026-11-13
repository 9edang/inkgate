# Stage 4 — endpoint

**Code:** [flow/endpoint/endpoint.go](../../flow/endpoint/endpoint.go) · **Entry point:** `endpoint.NewFactory`
· **Runs:** built once per endpoint, executed on every request

The business heart of the gateway. One request comes in, it is validated and transformed, sent to every backend
of the endpoint, and the answers are merged into one response.

```mermaid
flowchart TD
    IN([proxy.Request]) --> S1["1 OpenTelemetry span"] --> S2["2 pipeline metrics"]
    S2 --> S3["3 Lua (proxy): pre"]
    S3 --> S4{"4 CEL request rules"}
    S4 -- false --> XE([error])
    S4 --> S5{"5 JSON schema<br/>(request body)"}
    S5 -- invalid --> XE
    S5 --> S6["6 shadow: copy request to<br/>shadow backends, ignore result"]
    S6 --> D1
    subgraph CORE ["7 engine/proxy default pipeline"]
      D1["static data<br/>(strategy: always · success · errored · complete · incomplete)"] --> D2["req/resp modifier plugins"]
      D2 --> D3{"how many backends?"}
      D3 -- one --> B1[["stage 5: backend"]]
      D3 -- many --> M["merge<br/>parallel or sequential<br/>within 85% of timeout"]
      M --> B2[["stage 5: backend #1"]]
      M --> B3[["stage 5: backend #N"]]
      M --> F["flatmap_filter<br/>(move · append · del on arrays)"]
    end
    F --> OUT([proxy.Response])
    B1 --> OUT
```

Response-side work runs on the way back up: CEL response rules (4), Lua post scripts (3) and metrics/spans close.

## Steps

| # | Step | Code | Config |
|---|---|---|---|
| 1 | OpenTelemetry pipeline span | `feature/telemetry/otel/pipeline` | `telemetry/opentelemetry` (service) |
| 2 | pipeline metrics | `feature/telemetry/metrics` | `telemetry/metrics` (service) |
| 3 | Lua proxy script | `feature/transform/lua/proxy` | endpoint `modifier/lua-proxy` |
| 4 | CEL rules | `feature/validation/cel` | endpoint `validation/cel` |
| 5 | JSON schema | `feature/validation/jsonschema` | endpoint `validation/json-schema` |
| 6 | shadow backends | `engine/proxy/shadow.go` | backend `proxy.shadow: true`, `shadow_timeout` |
| 7 | default pipeline | `engine/proxy/factory.go` | see below |

## The default pipeline (step 7)

Built by `proxy.NewDefaultFactory` in `engine/proxy/factory.go`. Details are in [../engine/proxy.md](../engine/proxy.md).

- **Static data** (`proxy.static`): adds fixed data to the response, depending on the strategy. Useful as a fallback when backends fail.
- **Modifier plugins** (`plugin/req-resp-modifier`): Go `.so` plugins that can change the request and the response.
- **Merge** (only with 2+ backends): calls backends **in parallel** by default. With `proxy.sequential: true` they
  run one after another, and a later backend URL can use values from an earlier response (`{resp0_field}`).
  A custom `proxy.combiner` can replace the default merge.
  Missing or failed backends make the response *incomplete*. They don't cause an error.
- **Flatmap** (`proxy.flatmap_filter`): array operations on the merged data.

Each backend of the endpoint gets its own stage-5 pipeline.
