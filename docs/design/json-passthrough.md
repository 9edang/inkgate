# Design: JSON pass-through (M2 2.2)

Status: **proposal, for review**. Nothing here is implemented.

## Problem

A JSON backend answer is decoded into `map[string]any` and encoded again, even when nothing on the endpoint looks at
it. Measured in the [milestone baseline](../milestones/README.md#the-baseline-the-milestones-are-measured-against): a
100 KB answer costs 1,084 µs of CPU and 160 MB peak, 17× the 1 KB job. Most endpoints in a typical gateway do nothing
to the body: they authenticate, rate-limit and forward.

**Goal**: for those endpoints, send the backend's JSON bytes to the client without decoding and encoding them, with the
same headers, status and error behaviour the client gets today.

**Not a goal**: streaming (see "Buffer, do not stream"), non-JSON encodings, endpoints with several backends.

## Why not "use the `no-op` path when nothing reads the data"

That is the plan in M2 2.2. Checking the code shows two problems with it.

**1. "Nothing reads the data" cannot be decided by listing the readers.** `Response.Data` is read or changed by the
formatter (`allow`, `deny`, `mapping`, `group`, `target`, `flatmap_filter`, `is_collection`), CEL `resp_` rules, all
three Lua modules, request/response modifier plugins, the JWT signer (`auth/signer` signs fields of the body), martian
and the static proxy. Any new feature, and any plugin, can join that list. A block-list of readers is silently wrong
the day one is added: the endpoint keeps the fast path and the feature stops working without an error.

**2. The `no-op` path behaves differently, not only faster.** Measured against the JSON path:

| | JSON path today | `no-op` path |
|---|---|---|
| Headers sent to the client | the gateway's own (`Content-Type: application/json; charset=utf-8`, `X-Inkgate-Completed`) | every backend header, `Set-Cookie`, `Server` and internal ones included |
| Status | 200 for any 2xx | the backend's (201, 204...) |
| Invalid JSON from the backend | 500, nothing sent | forwarded to the client as it is |
| A JSON array, without `is_collection` | 500 (the body is decoded into an object) | forwarded |
| Failure while reading the body | 500, nothing sent | headers already sent: a 200 with a cut body |
| Key order | sorted (`JSON_SORT_MAP_KEYS`, default on) | the backend's |

Switching endpoints to `no-op` automatically would leak backend headers and change status codes and error handling
for every client. That is a different product, not an optimisation.

## Proposal

### Decide per endpoint at startup, with an allow-list

The fast path is taken only when **everything** on the endpoint is known to leave the body alone:

- exactly one backend, `encoding: json`, endpoint `output_encoding: json`;
- the backend has none of `allow`, `deny`, `mapping`, `group`, `target`, `is_collection`;
- no `sequential` proxy, no static response, no shadow backend;
- every `extra_config` namespace of the endpoint and the backend is on a **safe list** of namespaces that only touch
  the request or the raw HTTP exchange: logging and metrics, rate limits, the JWT validator, CORS, `security/http`,
  the bot detector, JSON-schema (request body), the circuit breaker, the HTTP cache, `backend/http`.

Anything else, including any namespace the gateway does not know (a plugin), keeps today's path. A feature added later
is safe by default: it is not on the list, so the endpoint keeps decoding. Adding a namespace to the list is a
deliberate change with its own test.

The decision is logged at DEBUG per endpoint (`[ENDPOINT: /users] JSON pass-through: on` or `off: mapping`), so an
operator can see it.

### Buffer, do not stream

Per request on a fast-path endpoint:

1. Read the backend body into one buffer, bounded by a size limit (the same default as `max_body_bytes`).
2. Validate it: a JSON object (the first non-space byte is `{`), valid as a whole. Validation scans without building
   anything (sonic's validator), far cheaper than decode and encode.
3. Write it with exactly today's headers and status: `Content-Type: application/json; charset=utf-8`,
   `X-Inkgate-Completed`, 200.

Invalid JSON, an array, an oversized body or a read error give the same 500 as today, before anything is sent.
Buffering keeps today's all-or-nothing behaviour and lets the backend connection go back to the pool before a slow
client is served. The cost is one copy of the body in memory instead of about 17.

Streaming would save that copy too, but a failure mid-body then reaches the client as a cut 200. That trade-off is for
a later, opt-in step.

### What clients can see

The body is the backend's bytes instead of a re-encoded copy. The value is the same; the text can differ:

- key order is the backend's, not sorted;
- whitespace and number formatting are the backend's (`1.0` stays `1.0`);
- integers above 2^53 keep their exact value (today they can lose precision through `float64`);
- HTML characters are not escaped (today they are only when `JSON_ESCAPE_HTML` is on).

JSON clients do not depend on these, but tests that compare bodies as strings will. That is why the rollout starts
opt-in.

## Rollout

1. **Opt-in**: a service setting `"router": {"json_passthrough": true}`. Measure, and run the integration suite with it
   on.
2. **Default on** one release later, with `"json_passthrough": false` to turn it off, after the opt-in has run without
   reports. The CHANGELOG states the visible differences above.

## Tests

- **Decision**: a table-driven test with one row per field and per namespace that must turn the fast path off, plus a
  row for an unknown namespace; and one per safe namespace that keeps it on.
- **Equivalence**: for a set of bodies (nested objects, unicode, large and small numbers, empty object), the fast path
  and today's path give the same status, the same headers and bodies that decode to the same value.
- **Failure**: invalid JSON, an array, a body over the limit and a read error all give today's 500, with nothing sent
  before.
- **Measure**: `proxy-json-100k` before and after; done when 3× or more, as in M2.

## Decisions for the reviewer

1. Allow-list instead of the block-list in M2 2.2: agreed?
2. Buffer (keep all-or-nothing), not stream: agreed?
3. Opt-in for one release, then default on: or default on immediately?
