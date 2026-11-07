# feature/transform — changing requests and responses

**Code:** [feature/transform](../../feature/transform): `lua`, `martian`, `xml`, `rss` (+ `internal/binder` Lua bridge, `internal/flatmap/tree`)

```mermaid
flowchart TD
    REQ((request)) --> L0["Lua service script<br/>(stage 2)"]
    L0 --> L1["Lua endpoint script<br/>modifier/lua-endpoint (stage 3)"]
    L1 --> L2["Lua proxy script<br/>modifier/lua-proxy (stage 4)"]
    L2 --> L3["Lua backend script<br/>modifier/lua-backend (stage 5)"]
    L3 --> M["Martian modifiers<br/>modifier/martian (stage 5 HTTP)"]
    M --> SVC[(backend)]
    SVC --> D["decode: xml / rss / json ..."]
    D --> L3
```

## lua — scripts at any stage

| Alias | Namespace | Where | Script sees |
|---|---|---|---|
| `modifier/lua-endpoint` | `inkgate/feature/transform/lua/router` | stage 2 (service) and stage 3 (endpoint); adapter `feature/transform/lua/router` | the HTTP request: `ctx` |
| `modifier/lua-proxy` | `inkgate/feature/transform/lua/proxy` | stage 4 | `request` and `response` objects |
| `modifier/lua-backend` | `inkgate/feature/transform/lua/proxy/backend` | stage 5 | `request` and `response` of one backend |

**Options:** `sources` (Lua files to load), `pre` (code run before the call), `post` (code run on the response),
`live` (reload sources on every request, for development), `allow_open_libs` (enable Lua's `io`, `os` and so on),
`skip_next` (answer from the script and don't call the next step), `md5` (checksums of the sources).

The Lua VM is `gopher-lua`, and Go types are exposed through `internal/binder`. Helpers available to scripts are in
`lua/decorator` (http, json, logging, etc.). A script can stop the request with `custom_error(msg, status)`.

## martian — declarative HTTP modifiers

| Namespace | `inkgate/feature/transform/martian` (alias `modifier/martian`), backend level |
|---|---|
| Where | wraps the HTTP client in stage 5 (`martian.NewConfiguredBackendFactory`) |
| Value | a Google Martian modifier tree, e.g. `header.Modifier`, `querystring.Modifier`, `body.Modifier`, `fifo.Group`, `cookie`, `stash`, `status`, `port`, `priority`, `url` |

Use it to add, rename or remove headers, query strings or the body without code.

## xml and rss — more encodings

Registered at boot (`flow/boot/encoding.go`):
- `xml`: backend `encoding: "xml"` decodes XML; endpoint `output_encoding: "xml"` renders XML (`feature/transform/xml`, registered by `entry.Router`).
- `rss`: backend `encoding: "rss"` decodes RSS/Atom feeds.

## flatmap (internal)

`internal/flatmap/tree` implements `flatmap_filter` operations (`move`, `append`, `del`) on nested arrays with `*`
wildcards. It is used by `engine/proxy/formatter.go` (see [../engine/proxy.md](../engine/proxy.md)).
