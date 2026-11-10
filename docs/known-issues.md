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

- **Unknown `extra_config` namespaces are ignored silently.** A typo turns a feature off without an error.
- **`check -l` needs `INKGATE_SCHEMA_URL`**, and `check -n` uses a placeholder schema (`schema/schema.json`
  is `{}`), so offline linting accepts everything until a schema is written.
- **inkgate supports a subset of the router options.** See [engine/gateway.md](engine/gateway.md).
  The others are ignored.
- **Rate-limit buckets are per instance**: N instances allow N × the configured rate.

## Follow-ups worth doing

1. Write a real JSON schema for `check -n` / `check -l`.
2. List the per-backend stack that `engine/proxy.NewDefaultFactory` builds (filters, load balancer, concurrency) as
   explicit steps in `flow/backend`.
3. Replace the name-based registries in `engine/register` (`sd`, `encoding`, renders) with plain switches.
