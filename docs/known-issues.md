# Known issues

## Unit tests that fail in a full `go test ./...` run

7 packages fail. None of them is caused by inkgate's code: the causes are missing test fixtures or test-environment
assumptions in carried-over tests. The integration suite passes.

| Package | Cause |
|---|---|
| `engine/cli` | `testdata/*-go.sum` fixtures for `check-plugin` were never shipped with the source |
| `engine/proxy`, `engine/proxy/plugin`, `engine/transport/http/{client,server}/plugin` | tests load `.so` plugins that must first be built with `go build -buildmode=plugin` (no make target yet) |
| `feature/auth/jose` | `TestJWK*` read a `cert.pem` fixture that was never shipped (`open cert.pem: no such file or directory`) |
| `feature/telemetry/metrics` | `TestMetrics_process` counts registry entries; with the pinned `rcrowley/go-metrics` it sees 6 instead of 42 when it runs after `TestMetrics`. Passes alone: `go test ./feature/telemetry/metrics -run TestMetrics_process`. Runtime behaviour is unaffected |

Occasionally flaky under parallel load (fixed local ports): `engine/transport/http/server`,
`feature/security/cors`. Both pass when run alone.

## Behaviour to be aware of

- **Unknown `extra_config` namespaces are ignored silently.** A typo turns a feature off without an error.
- **`check -l` needs `INKGATE_SCHEMA_URL`**, and `check -n` uses a placeholder schema (`flow/boot/schema/schema.json`
  is `{}`), so offline linting accepts everything until a schema is written.
- **inkgate supports a subset of the router options.** See [engines/mux.md](engines/mux.md).
  The others are ignored.
- **Rate-limit buckets are per instance**: N instances allow N × the configured rate.

## Follow-ups worth doing

1. Add a make target that builds the test `.so` plugins, so the plugin tests run.
2. Rewrite `TestMetrics_process` so it doesn't depend on test order.
3. Write a real JSON schema for `check -n` / `check -l`.
4. List the per-backend stack that `engine/proxy.NewDefaultFactory` builds (filters, load balancer, concurrency) as
   explicit steps in `flow/backend`.
5. Replace the name-based registries in `engine/register` (`sd`, `encoding`, renders) with plain switches.
