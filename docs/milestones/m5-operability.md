# M5: Config, observability and test hygiene

What makes the rest safer to change and easier to run. Several items are already listed under "Follow-ups worth
doing" in [../known-issues.md](../known-issues.md); this page gives them an order.

| # | Item | Size |
|---|---|---|
| 5.1 | Strict config: refuse unknown `extra_config` namespaces | S |
| 5.2 | A real JSON schema for `check -n` and `check -l` | M |
| 5.3 | Opt-in `pprof` endpoint | S |
| 5.4 | Performance regression check in CI | M |
| 5.5 | Make the whole test suite green | M |
| 5.6 | Config reload without a restart | L |

## 5.1 Strict config

An unknown namespace is ignored, so a typo turns a feature off without a word (`"qos/ratelimit/routr"`). Keep the
default, add `--strict` to `run` and `check` that fails on any namespace that no feature or alias knows, with a
"did you mean" from the alias list in `flow/boot/main.go`.

## 5.2 JSON schema

`flow/boot/schema/schema.json` is `{}`, so offline linting accepts everything. Generate the schema from the config
structs and each feature's option list, and test it against every config in `tests/fixtures`.

## 5.3 `pprof`

Every M2 item needs a CPU profile of a running gateway. An opt-in `debug/pprof` listener on its own address
(`"router": {"pprof_address": "127.0.0.1:6060"}`), never on the public port.

## 5.4 Performance regression check

The benchmark harness (`../benchmark`) can run a 5-minute pass (`make quick`). Run it in CI on a dedicated runner for
every change to `engine/` or `feature/`, compare against the last main-branch result, and fail when a job loses more
than 10% (run-to-run noise is under 5% on throughput). Store `results.json` per commit.

## 5.5 A green test suite

Today 7 packages fail in a full `go test ./...` for reasons outside the code (missing fixtures, unbuilt `.so`
plugins, an order-dependent test) and two are flaky on fixed ports. Build the test plugins from a make target, ship or
generate the missing fixtures (`testdata/*-go.sum`, `cert.pem`), use free ports in the server tests, and fix
`TestMetrics_process`. A green suite is what makes the M2 refactors safe.

## 5.6 Config reload (idea to discuss)

Reload the endpoints on SIGHUP or a file change without dropping connections: build the new router next to the old
one, then swap the handler. The parts that start background work at boot (async agents, metrics servers, the token
revoker) make this a design question before an implementation one.
