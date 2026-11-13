# M5: Config, observability and test hygiene

What makes the rest safer to change and easier to run. Several items are already listed under "Follow-ups worth
doing" in [../known-issues.md](../known-issues.md); this page gives them an order.

| # | Item | Size |
|---|---|---|
| 5.1 | Strict config: refuse unknown `extra_config` namespaces (**done**) | S |
| 5.2 | A real JSON schema for `check -n` and `check -l` (**done**) | M |
| 5.3 | Opt-in `pprof` endpoint | S |
| 5.4 | Performance regression check in CI | M |
| 5.5 | Make the whole test suite green (**done**) | M |
| 5.6 | Config reload without a restart | L |

## 5.1 Strict config

An unknown namespace is ignored, so a typo turns a feature off without a word (`"qos/ratelimit/routr"`). Keep the
default, add `--strict` to `run` and `check` that fails on any namespace that no feature or alias knows, with a
"did you mean" from the alias list in `flow/boot/main.go`.

**Result** (November 2026): strict by default, not behind `--strict`, because the silent default is the bug.
`run` and `check` refuse a namespace that is neither an alias, a full name nor one of the four that have no alias
(`qos/ratelimit/router`, `qos/ratelimit/proxy`, `telemetry/opentelemetry`, `backend/http/client`), naming the endpoint
and the closest known name;
`--allow-unknown-namespaces` is the way out for configs migrated from KrakenD. `router/stream` also refuses a value
that is not an object (`true` used to turn event mode on) and an unknown option, and `check` runs that validation too.
The known list is tested against the code: a test reads every `Namespace` constant and every key read off an
`ExtraConfig` field and fails on one that is not listed, which is how `backend/http/client` (a constant not called
`…Namespace`) was found. The level a namespace is written at is only checked by the schema (`check -n`), not by `run`.

## 5.2 JSON schema

`schema/schema.json` is `{}`, so offline linting accepts everything. Generate the schema from the config
structs and each feature's option list, and test it against every config in `tests/fixtures`.

**Result** (November 2026): `schema/schema.json` (JSON Schema 2020-12, embedded for `check -n`) describes the service,
endpoints, backends, async agents, TLS and every namespace with its options, and where each may be written (service,
endpoint, backend), from `docs/guides/config-reference.md` and the code of each feature. Unknown keys are refused at
every level. Tests (`schema/schema_test.go`): every config of the repository and the examples of the docs pass; one
paired good and bad document per kind of mistake (typo in a service, endpoint or backend key, a namespace name, an
option, a namespace at the wrong level, a number for a duration, a bad enum), so a refusal is about that one change;
and `flow/boot` checks that the schema lists exactly the known namespaces. Written by hand from the code, not generated,
so a new option has to be added to the schema too: the sync test catches a new namespace, not a new option.

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

**Result** (November 2026): `make test` is green, also under `-race` and in `golang:1.27-trixie` without network.
`make test` builds the test plugins (`test-plugins`) and the TLS fixtures (`test-certs`); the server tests use free
ports; seven data races, a flaky proof-of-work test and an integration case that called the internet were fixed. What
is left to know is in [../known-issues.md](../known-issues.md); the checks to run before a PR are in
[../guides/testing.md](../guides/testing.md#before-opening-a-pr).

## 5.6 Config reload (idea to discuss)

Reload the endpoints on SIGHUP or a file change without dropping connections: build the new router next to the old
one, then swap the handler. The parts that start background work at boot (async agents, metrics servers, the token
revoker) make this a design question before an implementation one.
