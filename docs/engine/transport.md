# engine/transport — HTTP in and out

**Code:** [engine/transport/http/server](../../engine/transport/http/server), [engine/transport/http/client](../../engine/transport/http/client)

```mermaid
flowchart LR
    subgraph IN ["server (inbound)"]
      S1["RunServerWithLoggerFactory<br/>net/http.Server"] --> S2["TLS / mTLS<br/>tls.*"]
      S1 --> S3["h2c<br/>use_h2c"]
      S1 --> S4["graceful shutdown<br/>max_shutdown_wait_time"]
      S1 --> SP["server/plugin<br/>http-server .so"]
    end
    subgraph OUT ["client (outbound)"]
      C1["NewHTTPClient<br/>shared transport"] --> C2["status handler<br/>status.go"]
      C1 --> C3["client_tls<br/>CA, client certs"]
      C1 --> CP["client/plugin<br/>http-client .so"]
      C1 --> GQ["client/graphql"]
    end
```

## Server

| Setting | Meaning |
|---|---|
| `port`, `listen_ip` | where to listen (default `:8080`) |
| `tls` | `public_key`/`private_key` or `keys[]`, `min_version`, `max_version`, `cipher_suites`, `curve_preferences`, `enable_mtls`, `ca_certs`, `disable_system_ca_pool` |
| `use_h2c` | HTTP/2 without TLS |
| `read_header_timeout`, `read_timeout`, `idle_timeout`, `write_timeout`, `max_header_bytes`, `max_body_bytes` | request limits, see below |
| `max_shutdown_wait_time` | how long in-flight requests may finish after SIGTERM |

### Request limits

Every server gets limits against slow and oversized requests, unless the config sets its own. A negative value
turns a limit off (`"read_timeout": "-1s"`, `"max_body_bytes": -1`).

| Setting | Default | Bounds |
|---|---|---|
| `read_header_timeout` | 10s | the time to send the request headers (slow-client attacks) |
| `read_timeout` | 30s | the time to send the whole request, body included. Streamed responses (SSE) and WebSocket connections are not cut: Go clears the deadline when a connection is hijacked |
| `idle_timeout` | 120s | how long an idle keep-alive connection is kept |
| `write_timeout` | none | it would cut long responses and streams; use the endpoint `timeout` instead |
| `max_header_bytes` | 1 MB | the size of the request line and headers |
| `max_body_bytes` | 10 MB | the request body. A larger declared body gets 413 before any backend is called; a chunked body is cut when it passes the limit, and the client gets 413 |

The limits apply to the whole service. If it accepts large uploads over slow links, raise `read_timeout` and
`max_body_bytes` together: 10 MB at 2 Mbit/s already takes about 40 seconds.

`DefaultToHTTPError` maps pipeline errors to 500, except for errors that carry their own status code.

## Client

One `http.Transport` is shared by all backends (`NewTransport`), tuned by the service settings
`max_idle_connections`, `max_idle_connections_per_host`, `idle_connection_timeout`, `response_header_timeout`,
`expect_continue_timeout`, `dialer_timeout`, `dialer_keep_alive`, `dialer_fallback_delay`, `disable_keep_alives`,
`disable_compression`, `client_tls`, `allow_insecure_connections`.

### Status handling (`backend/http` alias, `inkgate/engine/transport/http/client`)

| Option | Effect when the backend answers non-2xx |
|---|---|
| none | only 200 and 201 count as success; anything else is an error and the backend is dropped from the merge |
| `return_error_code: true` | the client gets the backend's status code |
| `return_error_details: "name"` | the backend status and body go into the response under the key `error_<name>` |

### GraphQL (`backend/graphql`)

`type` (`query`|`mutation`), `query` or `query_path`, `variables`, `operation`, `method`. The body sent to the
backend is built from these, and the endpoint's params can fill `variables`.
