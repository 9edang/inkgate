# Upstream: where inkgate comes from and how to follow it

inkgate is a fork. Its code derives from [Lura](https://github.com/luraproject/lura) and
[KrakenD CE](https://github.com/krakend/krakend-ce) and the component libraries CE is built from (Apache-2.0, see
[NOTICE](../NOTICE)). **Nobody upstream fixes a bug in inkgate's copy.** A security fix in Lura or in a KrakenD
library reaches inkgate only if someone reads it and ports it. This page says what the code was forked from, how far
each package has moved, how to follow upstream, and which features are candidates for removal.

## 1. What it was forked from

The repository starts with one commit that imports the finished code (`207bc69`, 626 files), so the upstream history
and versions were not recorded. The baseline below was **reconstructed on 13 November 2026 by comparing the code**
with upstream releases (all `.go` files without tests, import blocks and comments removed, lines compared).

| | Baseline | How sure |
|---|---|---|
| Lura | `v2.14.2-0.20260316170719-6d79b4ef723b` (commit `6d79b4ef723b`, 16 March 2026, after the tag `v2.14.1`) | high. `engine/async` uses `math.MaxInt`, which that commit has (`v2.14.1` has `math.MaxInt64`); `max_shutdown_wait_time` and `RegisterExtraComponents` are in (added in 2.13.0 and 2.14.x); of the one file that differs between `v2.14.1` and this commit (`async/asyncagent.go`) inkgate has the commit's version, and `proxy/factory.go` is identical to it |
| KrakenD CE | `v2.13.x`, most likely `v2.13.11` | medium. CE 2.13.11 pins exactly the Lura commit above; the component libraries below are the versions its `go.mod` pins, and each inkgate package is 75 to 100% line for line the same as that version (table in section 2) |

The component versions the code matches (from the `go.mod` of `krakend-ce v2.13.11`):

| Library | Version | inkgate package |
|---|---|---|
| `krakend-jose/v2` | v2.12.3 | `feature/auth/jose` |
| `go-auth0/v2` | v2.0.4 | `internal/auth0` |
| `bloomfilter/v2` | v2.1.0 | `feature/auth/bloomfilter` |
| `krakend-oauth2-clientcredentials/v2` | v2.2.0 | `feature/auth/oauth2client` |
| `krakend-botdetector/v2` · `krakend-cors/v2` · `krakend-httpsecure/v2` | v2.3.1 · v2.2.1 · v2.2.1 | `feature/security/*` |
| `krakend-otel` | v0.14.5 | `feature/telemetry/otel` |
| `krakend-metrics/v2` · `krakend-influx/v2` · `krakend-gelf/v2` · `krakend-logstash/v2` · `krakend-gologging/v2` | v2.2.0 · v2.2.0 · v2.1.0 · v2.1.0 · v2.1.1 | `feature/telemetry/*` |
| `krakend-audit` | v0.9.3 | `feature/telemetry/audit` |
| `krakend-ratelimit/v3` · `krakend-circuitbreaker/v3` · `krakend-httpcache/v2` | v3.3.1 · v3.0.0 · v2.2.0 | `feature/traffic/*` |
| `krakend-lua/v2` · `krakend-martian/v2` · `krakend-rss/v2` · `krakend-xml/v2` | v2.9.1 · v2.3.0 · v2.1.1 · v2.2.2 | `feature/transform/*` |
| `krakend-cel/v2` · `krakend-jsonschema/v2` | v2.4.1 · v2.2.1 | `feature/validation/*` |
| `krakend-amqp/v2` · `krakend-pubsub/v2` · `krakend-lambda/v2` | v2.3.1-0.20260317155713-585835a83dca · v2.3.1 · v2.1.0 | `feature/connector/*` |
| `krakend-cobra/v2` · `krakend-koanf` · `krakend-flexibleconfig/v2` | v2.8.2 · v0.1.1 · v2.3.1 | `engine/cli`, `engine/config/koanf`, `engine/config/flexibleconfig` |
| `binder`, `flatmap` | v0.0.0-20250826131726-e91a8a754ef8, v1.2.0 | `internal/binder`, `internal/flatmap` |

If you know the real fork point, correct this table: it is the starting point for every comparison below.

## 2. How far each package has moved

"Unchanged" is the share of inkgate's lines that also appear, in the same order, in the matching upstream file.
Measured on the working tree with the dependency changes of November 2026 (OpenCensus removed, Lambda on the AWS SDK
v2, go-jose v4, `log/slog` logging), so those packages read lower than they were at the fork.

- **Close** (90% or more): a security fix upstream applies almost as it is; port by diff.
- **Modified** (75 to 90%): port by reading the upstream change and applying it by hand.
- **Diverged** (under 75%): treat upstream as a reference, not as a source of patches.

### Close

| inkgate | Upstream | Unchanged | Note |
|---|---|---|---|
| `engine/async`, `engine/plugin`, `feature/transform/rss`, `internal/flatmap` | Lura, rss, flatmap | 100% | identical |
| `feature/validation/cel` | `krakend-cel` | 99.6% | |
| `engine/encoding`, `feature/auth/oauth2client`, `feature/traffic/httpcache` | Lura, oauth2, httpcache | 98.5 to 99.1% | |
| `engine/config/koanf`, `engine/sd` | koanf, Lura | 97.6 to 97.8% | |
| `feature/telemetry/audit` | `krakend-audit` | 97.5% | reads KrakenD configs; still flags OpenCensus |
| `feature/traffic/circuitbreaker` (`gobreaker`) | `krakend-circuitbreaker` | 97.5% | |
| `feature/connector/amqp` | `krakend-amqp` | 96.3% | |
| `feature/security/httpsecure`, `feature/telemetry/influx`, `feature/telemetry/gelf` | | 95.6 to 96.2% | |
| `engine/cli` | `krakend-cobra` | 95.2% | `run` and `check` flow differs |
| `engine/register`, `feature/transform/martian`, `feature/security/botdetector` | | 95% | |
| `feature/telemetry/otel` | `krakend-otel` | 94.4% | 33 files; 14 changed |
| `engine/config/flexibleconfig`, `engine/proxy`, `internal/auth0` | | 93.4 to 94.3% | `engine/proxy` has `passthrough.go` of its own |

### Modified

| inkgate | Upstream | Unchanged | What differs (where known) |
|---|---|---|---|
| `feature/transform/xml` | `krakend-xml` | 89.7% | |
| `internal/binder` | `binder` | 88.5% | |
| `engine/logging` | Lura | 87.1% | `Enabled` level check added |
| `feature/auth/bloomfilter` | `bloomfilter` | 87.0% | `gateway.go` is new |
| `feature/connector/pubsub`, `feature/telemetry/logstash`, `feature/validation/jsonschema` | | 85.3 to 85.9% | |
| `feature/auth/jose` | `krakend-jose` | 82.9% | go-jose v4 (upstream did the same in `krakend-jose` 2.12.4, see section 3), `audience` kept as "every one" |
| `engine/transport/http/server` | Lura | 82.0% | `drain.go`: graceful drain of streams and sockets |
| `feature/transform/lua` | `krakend-lua` | 81.2% | `compiled.go`, `pool.go` are new |
| `engine/transport/http/client` | Lura | 79.3% | |
| `feature/traffic/ratelimit` | `krakend-ratelimit` | 79.0% | own `limiter.go`; Gin glue dropped |
| `engine/config` | Lura | 75.5% | `number.go`, `template.go` are new |
| `feature/connector/lambda` | `krakend-lambda` | 75.0% | AWS SDK v2, `max_retries` fixed |

### Diverged

| inkgate | Upstream | Unchanged | What differs |
|---|---|---|---|
| `engine/core` | Lura | 60% | 10 lines; version strings |
| `feature/telemetry/metrics` | `krakend-metrics` | 51.6% | stream and socket metrics (`stream.go`), router glue |
| `feature/telemetry/gologging` | `krakend-gologging` | 42.9% | rewritten on `log/slog`, same pattern syntax |
| `feature/security/cors` | `krakend-cors` | 41.2% | |
| `engine/router` | Lura `router/mux` | 23.5% | not a port: SSE and WebSocket (`router/stream`), origin and subprotocol policy, content negotiation |
| `flow/` | `krakend-ce` root | 4.9% | rewritten as the staged pipeline ([flow/README.md](../flow/README.md)); nothing to port by diff |

Everything not listed (for example `engine/proxy/plugin`, `context`, `internal/json`, `schema`) has no upstream
counterpart in these libraries, or was added by inkgate.

## 3. How to follow upstream

### What `govulncheck` does and does not cover

`make check-all` runs `govulncheck`. It reports known vulnerabilities **in modules inkgate imports** (go-jose, the AWS
SDK, `gorilla/mux`, the Go standard library...). It does **not** see the forked code: inkgate's copy of `krakend-jose`
is `github.com/9edang/inkgate/feature/auth/jose`, not a module, so a CVE in `krakend-jose` is invisible to it. That is
what this section is for.

### What to watch

| Watch | Why | Where |
|---|---|---|
| Lura releases and commits on `master` | the proxy, router, config, transport code | `github.com/luraproject/lura/releases`, `/commits/master` |
| `krakend-ce` releases | CE's `go.mod` says which version of every library is current; CE's changelog names the fixes | `github.com/krakend/krakend-ce/releases` |
| The libraries behind the most exposed code: `krakend-jose` and `go-auth0` (JWT), `krakend-ratelimit`, `krakend-botdetector`, `krakend-cors`, `krakend-httpsecure`, `krakend-cel`, `krakend-lua`, `krakend-jsonschema`, `krakend-otel` | a bug there is reachable from the network | `github.com/krakend/<name>/releases` |
| GitHub security advisories of `luraproject/lura` and `krakend/*` | the only place a fix is called a security fix | `/security/advisories` |
| The Go release notes and `go-jose` | the toolchain and the one JWT library are directly imported, so `govulncheck` covers them, but a release is the signal to rebuild plugins ([guides/custom-plugins.md](guides/custom-plugins.md)) | `go.dev/doc/devel/release`, `github.com/go-jose/go-jose/releases` |

### How often

- **Every `krakend-ce` release** (about monthly): compare its `go.mod` with the baseline in section 1. Any library
  whose version moved is a candidate to read.
- **Weekly**, a quick look at the security advisories above, and at `go list -m -versions` for the libraries in the
  table of section 1.
- **Immediately**, on a public advisory for Lura or a library inkgate carries.

### How to read one release

```sh
# what is new in an upstream library since the baseline
go mod download github.com/krakend/krakend-jose/v2@v2.12.3 github.com/krakend/krakend-jose/v2@v2.12.5
diff -ru "$(go env GOMODCACHE)/github.com/krakend/krakend-jose/v2@v2.12.3" \
         "$(go env GOMODCACHE)/github.com/krakend/krakend-jose/v2@v2.12.5" -x '*_test.go' -x go.sum

# the latest version of each library
go list -m -versions github.com/krakend/krakend-jose/v2 | tr ' ' '\n' | tail -1
```

If a change touches code inkgate carries, port it by hand, add a test for it, and update the baseline in section 1.
Record what was skipped and why in the changelog.

### The gap on 13 November 2026

| Library | Baseline | Latest | What the new release holds |
|---|---|---|---|
| `krakend-jose/v2` | v2.12.3 | v2.12.5 | moves to go-jose v4.1.6 and `go-auth0` 2.0.5. inkgate already runs on go-jose v4, so the migration is done; **upstream accepts any one of the configured audiences, inkgate requires every one** (kept from v3 on purpose, see the changelog) |
| `go-auth0/v2` | v2.0.4 | v2.0.5 | go-jose v4, the parse-time algorithm list and `AnyAudience`; inkgate has the same list and keeps "every audience" |
| `krakend-audit` | v0.9.3 | v0.9.4 | a rule now also counts `qos/circuit-breaker/http` |
| Lura | the commit above | v2.14.1 (a tag older than the baseline) | nothing newer |
| `krakend-ce` | v2.13.x | v2.13.11 | none |
| `go-jose/v4` (imported directly) | v4.1.4 | v4.1.9 | not checked for fixes; `govulncheck` reports none that inkgate calls. Worth a bump and a read of the notes |

## 4. Removal candidates

Every feature is a copy that inkgate maintains alone. These are the ones whose cost is highest or whose use is least
likely, each with the reason. Nothing here is decided.

| Candidate | Reason | Cost of removing it |
|---|---|---|
| `feature/traffic/circuitbreaker/eapache` (2 packages) | not linked into the binary: no flow builds it, `qos/circuit-breaker` is the `gobreaker` one. | none; it is dead code |
| `feature/transform/martian` | the upstream `google/martian` is archived (last push December 2024) and pinned at a 2019 commit; 20 packages and about 7,800 lines are linked in, 3,000 of them a proxy and TLS-interception stack inkgate never calls. Only 4 of its 25 modifiers have a test | breaking for any config with `modifier/martian`; the alternative is to vendor about 4,500 lines. A decision is pending |
| `feature/telemetry/influx` | KrakenD's own audit flags it as deprecated (`hasDeprecatedInflux`); OpenTelemetry is the supported metrics path | breaking for configs with `telemetry/influx` |
| `feature/connector/pubsub` | one connector brings a large dependency tree (`gocloud.dev` and the cloud SDKs behind it); no test under `tests/` uses it | breaking for configs with `backend/pubsub/*` |

Already removed: OpenCensus (the exporters and its three spans) and the usage reporter. The reasons are in
[../CHANGELOG.md](../CHANGELOG.md).

To rank the rest, look at what `deadcode ./cmd/inkgate` reports as unreachable: it lists functions nothing calls, and
the largest groups are in `engine/proxy`, `engine/config`, `internal/auth0` and `engine/cli`.
