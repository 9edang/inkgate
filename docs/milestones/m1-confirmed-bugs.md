# M1: Fix confirmed bugs

Each bug below was reproduced while building the benchmark. They are small, but each one gives a wrong answer to a
correct config today.

| # | Bug | Where | Size |
|---|---|---|---|
| 1.1 | CEL `resp_metadata_status` is always 0 for decoded backends | `feature/validation/cel/proxy.go`, `engine/proxy/http_response.go` | S |
| 1.2 | A CORS preflight is refused when `allow_headers` is set | `feature/security/cors/http.go` (rs/cors v1.11.1) | S |
| 1.3 | The documented Lua setter `resp:data(key, value)` does nothing | `feature/transform/lua/proxy/response.go`, docs | S |
| 1.4 | `plugin/http-client` chains buffer the whole response | `engine/transport/http/client/plugin/executor.go` | M |

## 1.1 CEL `resp_metadata_status` is always 0

**Seen**: a rule `resp_metadata_status == 200` on an endpoint with a normal JSON backend fails every request with a 500.
`resp_completed == true` on the same endpoint works.

**Cause**: `cel/proxy.go:128` reads `r.Metadata.StatusCode`, but `DefaultHTTPResponseParserFactory`
(`engine/proxy/http_response.go`) builds `Response{Data: data, IsComplete: true}` and never fills `Metadata`. Only the
`no-op` parser and error responses set it. So every rule on the status of a decoded response compares against 0.

**Fix**: set `Metadata.StatusCode` (and `Headers`) in the default parser from `resp`. Check what the merge does
with the metadata of several backends before choosing which status a merged response reports.

**Done when**: a test with `resp_metadata_status == 200` passes on a decoded backend, and the merge rule is written in
[../engine/proxy.md](../engine/proxy.md).

## 1.2 CORS preflight refused with `allow_headers`

**Seen**: with `"allow_headers": ["Origin", "Content-Type"]` (any spelling or case), a preflight with
`Access-Control-Request-Headers: Content-Type` gets a 204 **without** `Access-Control-Allow-Origin`, so the browser
blocks the real request. The CORS debug log says `Preflight aborted: headers '[Content-Type]' not allowed`.

**Cause**: reproduced with `rs/cors` v1.11.1 alone, outside the gateway: only `AllowedHeaders: ["*"]` passes. The
gateway's config parsing is correct. The same happens in KrakenD CE, which uses the same library.

**Fix**: first look for the upstream issue or a newer release. If none, normalize the list before passing it on
(lower case, trimmed), and add a preflight test with an explicit header list. Until then, document it in
[../feature/security.md](../feature/security.md): leave `allow_headers` empty (it then means `*`).

**Done when**: a preflight test with an explicit `allow_headers` returns the allow-origin header.

## 1.3 Lua `resp:data(key, value)` does nothing

**Seen**: `local resp = response.load(); resp:data('lua', 'yes')` runs without error but the field never appears.
`local d = resp:data(); d:set('lua', 'yes')` works.

**Cause**: `ProxyResponse.data` (`lua/proxy/response.go`) only returns the table; the two-argument form is ignored.
The old form is still used in `lua/proxy/proxy_benchmark_test.go` and in examples copied from it.

**Fix**: either support the setter (when called with two arguments, set the key) or raise an error naming the
`:set` form. Supporting it keeps old scripts working.

**Done when**: both forms add the field, with a test each, and [../feature/transform.md](../feature/transform.md)
shows the table API.

## 1.4 `plugin/http-client` chains buffer the whole response

**Seen**: the chain runs the plugins into an `httptest.ResponseRecorder` (`executor.go:125`) and only returns once
the whole body is written. A backend called through a client plugin cannot stream (`router/stream` gets one buffered
body), and every response is copied once more in memory.

**Fix**: run the chain on an `io.Pipe`-backed `http.ResponseWriter` that returns the `*http.Response` as soon as the
headers are written (status and headers first, then the body streams through the pipe), and pass flushes through.

**Done when**: an SSE backend behind a client plugin passes the stream tests in `engine/router/stream_test.go`,
and the existing chain tests still pass.
