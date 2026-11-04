# Request lifecycle

What happens to one request, from the client to the backends and back. Each box names the stage folder in `flow/`
and the step number used in the code comments.

## Sequence

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant E as entry<br/>listener + gorilla/mux
    participant R as request<br/>endpoint handler
    participant P as endpoint<br/>endpoint pipeline
    participant B as backend<br/>backend call (×N)
    participant S as Backend service

    C->>E: HTTP request
    Note over E: flow context → OTel server span → CORS → http-server plugins<br/>secure headers → Lua (service) → bot detector
    E->>E: router matches endpoint + method
    alt no route / wrong method
        E-->>C: 404 / 405 (optional custom error_body)
    end
    E->>R: handler of the matched route
    Note over R: OTel → bot detector → OpenCensus → metrics<br/>JWT validate + revocation → Lua (endpoint) → rate limit
    alt rejected (bot, bad JWT, over limit)
        R-->>C: 403 / 401 / 429 / 503 ...
    end
    R->>R: build proxy.Request<br/>(only allowed headers and query strings)
    R->>P: proxy(ctx, request)
    Note over P: OTel → OpenCensus → metrics → Lua (proxy)<br/>CEL → JSON schema → shadow → static → modifier plugins
    par each backend (parallel, or sequential if configured)
        P->>B: backend proxy
        Note over B: filters → URL build → load balance → concurrent calls<br/>OTel → metrics → circuit breaker → rate limit → Lua → CEL
        alt Lambda / AMQP / Pub/Sub backend
            B->>S: connector call
        else HTTP backend
            B->>S: Martian modifiers → HTTP client (cache, OAuth2)
        end
        S-->>B: response
        B-->>P: decoded data (allow/deny, mapping, group, target)
    end
    P->>P: merge (within 85% of endpoint timeout) → flatmap
    P-->>R: proxy.Response (Data, IsComplete, Metadata)
    R-->>C: rendered response + X-Inkgate-Completed header
```

## The data on the way

| Point | Type | Defined in |
|---|---|---|
| entering stage 3 | `*http.Request` (+ `mux.Vars`) | net/http, gorilla/mux |
| between 3 and 4 | `*proxy.Request`: Method, URL, Query, Path, Body, Params, Headers | `engine/proxy/request.go` |
| out of 4 and 5 | `*proxy.Response`: Data (map), IsComplete, Metadata (headers, status), Io (raw body for `no-op`) | `engine/proxy/proxy.go` |
| a backend step | `proxy.Proxy` = `func(ctx, *Request) (*Response, error)` | `engine/proxy/proxy.go` |

## Completeness

`IsComplete` is true only when every backend answered without error before the timeout. The handler sends it as
the `X-Inkgate-Completed` header. A merge with failed backends still returns what it has: status 200 with partial
data, unless the endpoint uses `no-op` encoding, or a backend error is turned into the response by
`return_error_code` / `return_error_details` (see [../engine/transport.md](../engine/transport.md)).

## Where time goes

- `timeout` on the endpoint (default from the service `timeout`) bounds the whole pipeline.
- The merge step gives its backends **85%** of that timeout, so there is time left to render.
- Shadow backends have their own `shadow_timeout` and never delay the response.
