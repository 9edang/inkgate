# Known issues

## Test suite

`make test` is green: every unit package, the integration suite (`tests/`) and the AMQP suite (`tests/amqp`).

- `tests/amqp` uses testcontainers and **skips** (it does not fail) when no container engine is reachable. With
  podman: `systemctl --user start podman.socket`, then run with
  `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock TESTCONTAINERS_RYUK_DISABLED=true`.
- Go 1.27's `encoding/json` (json v2) words some errors differently from older Go (`in string` instead of
  `in string literal`). Tests assert on the stable part of library messages only.
- `internal/json` decodes whole numbers into `any` as `int64` (`JSON_USE_INT64`), `encoding/json` and koanf as
  `float64`. Code reading a number from `extra_config` uses `config.Float64`, which accepts both.

## Behaviour to be aware of

- **`check` finds unknown namespaces and `router/stream` mistakes; `check -n` finds the rest.** A typo in an option
  inside another namespace (`"max_rte"` in `qos/ratelimit/router`), a misspelled top-level, endpoint or backend key and
  a namespace written at a level that does not read it are accepted by `run` and by plain `check`, and the feature is
  silently off. Run `check -n` before a deploy.
- **`check -l` needs `INKGATE_SCHEMA_URL`.** `check -n` uses the schema in the binary.
- **inkgate supports a subset of the router options.** See [engine/gateway.md](engine/gateway.md).
  The others are ignored.
- **Rate-limit buckets are per instance**: N instances allow N × the configured rate.
- **With 500 routes inkgate is slower than KrakenD CE** (the `routes` scenario: 24,712 against 26,897 req/s, 0.92x, ranges
  not overlapping, while it leads in every other scenario). The router is gorilla/mux, which tries its routes in order
  and matches each with a regular expression, so the cost grows with the number of routes. This is the likely cause, not
  yet measured with a profile; see [benchmark/](../benchmark/README.md).

## Follow-ups worth doing

2. List the per-backend stack that `engine/proxy.NewDefaultFactory` builds (filters, load balancer, concurrency) as
   explicit steps in `flow/backend`.
3. Replace the name-based registries in `engine/register` (`sd`, `encoding`, renders) with plain switches.
