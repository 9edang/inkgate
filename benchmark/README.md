# benchmark: inkgate vs KrakenD CE

Both gateways run in containers with the same config, the same CPU and memory limits and the same load generator.
They run one at a time, so they never compete for the machine.

```
oha (load) ──► gateway (inkgate or KrakenD CE) ──► nginx (fixed JSON answers)
```

| File | What it is |
|---|---|
| [run.sh](run.sh) | builds the inkgate image, runs every scenario against both gateways, then calls report.py |
| [config/gateway.json](config/gateway.json) | the one config both gateways load for the proxy scenarios (inkgate keeps the KrakenD format) |
| [config/plugin-*.json](config) | the same for the plugin scenarios: one file per scenario, loaded by both gateways |
| [plugins/](plugins) | the plugin sources, see "Plugins" below |
| [backend/nginx.conf](backend/nginx.conf) | the upstream: `/ping`, `/user`, `/orders` with fixed bodies |
| [report.py](report.py) | turns the raw numbers into `results/report.md`, `summary.json` and `rps.svg` |
| [results/](results/) | the last run: [report.md](results/report.md), `summary.json`, `meta.json`, `rps.svg`, `raw/` |

## Scenarios

| Endpoint | Backends | What it measures |
|---|---|---|
| `/passthrough` | 1, `no-op` encoding | the cost of proxying: the body is streamed, never parsed |
| `/json` | 1, JSON | proxying plus decoding and encoding the JSON again |
| `/merge` | 2, JSON | calling two backends in parallel and merging both answers |
| `/plugin-modifier` | 1, JSON | a request modifier adds a header for the backend, a response modifier adds a field to the body |
| `/plugin-server` | 1, JSON | a server plugin wraps the HTTP handler and adds a response header |
| `/plugin-client` | 1, JSON | a client plugin replaces the backend client and calls the API itself through its own pooled `http.Client` |

The plugin scenarios are reported against `/json` (the same request without a plugin), so the report shows what the
plugin costs, per gateway.

## Plugins

Go plugins (`.so`) load only into a binary built with the same toolchain, flags and packages, so they are built once per
gateway by `run.sh`: the inkgate ones in the Dockerfile build stage, the KrakenD ones with `krakend/builder`.

| Source | Plugin type | Notes |
|---|---|---|
| [plugins/modifier](plugins/modifier) | `plugin/req-resp-modifier` | standard library only, one source for both gateways |
| [plugins/server](plugins/server) | `plugin/http-server` | standard library only, one source for both gateways |
| [plugins/client-krakend](plugins/client-krakend) | `plugin/http-client` | KrakenD style: plain signature, standard library only |
| [plugins/client-inkgate](plugins/client-inkgate) | `plugin/http-client` | inkgate style: chainable signature (receives `next`), inkgate `context`, built inside the repo module so it has no `go.mod` |

The two client plugins do the same thing. Before a scenario is measured, `run.sh` requests it once and checks that the
plugin took effect (a header or field in the answer); a broken plugin would otherwise answer errors quickly and look fast.

## Same spec

- **Config**: one file per scenario, mounted into both. Logging is `ERROR` only; no metrics, no CORS, no cache.
- **Limits**: each gateway gets `GATEWAY_CPUS=2` and `GATEWAY_MEMORY=512m`; the backend 2 CPUs; the load generator 4 CPUs.
- **Load**: 128 keep-alive connections, 10 s warm-up (not measured), then 30 s measured, 3 rounds. The order of the
  two gateways alternates between rounds, and the median round (by requests per second) is reported.
- **Versions**: KrakenD CE `2.13.11`, nginx `1.31.8`, oha `1.16.0`, inkgate built from this checkout. Pinned in
  [run.sh](run.sh).
- Requests still in flight when a run ends are aborted by the load generator; they are not counted as failures.

## Run it

```sh
./benchmark/run.sh                          # or: make bench
DURATION=60s ROUNDS=5 ./benchmark/run.sh    # longer runs
SKIP_BUILD=1 ./benchmark/run.sh             # reuse the inkgate image from the last run
DOCKER=podman ./benchmark/run.sh            # pick the container CLI (docker by default)
```

All settings (`CONNECTIONS`, `GATEWAY_CPUS`, `GATEWAY_MEMORY`, image names, ...) are environment variables at the top
of [run.sh](run.sh). Needs `docker` or `podman`, and `python3`.

Absolute numbers depend on the machine. Compare the two gateways within one run, not across machines.

What to improve in this harness: [docs/guides/benchmarking.md](../docs/guides/benchmarking.md).
