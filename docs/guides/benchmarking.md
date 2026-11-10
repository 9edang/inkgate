# Benchmarking

The harness is in [benchmark/](../../benchmark/README.md). It runs inkgate and KrakenD CE in containers with the same
config, the same limits and the same load generator, and writes
[benchmark/results/report.md](../../benchmark/results/report.md). Run it with `make bench`.

## Current setup

| | |
|---|---|
| Gateways | inkgate (built from the checkout) and KrakenD CE 2.13.11, one at a time |
| Limits | each gateway `--cpus 2 --memory 512m`; nginx backend 2 CPUs; `oha` 4 CPUs; all on one host |
| Load | 128 keep-alive connections, 10 s warm-up, 30 s measured, 3 rounds, median reported |
| Scenarios | passthrough, json, merge, plugin-modifier, plugin-server, plugin-client |
| Bodies | small fixed JSON (under 200 bytes) |
| Config | logging at `ERROR`, no metrics, no CORS, no rate limit |

## Improvements

Most important first. The status column is where the harness stands today.

| # | Improvement | Status |
|---|---|---|
| 1 | More rounds and spread | todo |
| 2 | Pin CPUs | todo |
| 3 | GOMAXPROCS=2 on both gateways | already the default, make it explicit |
| 4 | Document the memory choice | todo |
| 5 | Vary payload size | todo |
| 6 | Vary concurrency | todo |
| 7 | Report errors | partly done |
| 8 | Record the host state | partly done |
| 9 | Separate host for load and backend | todo |
| 10 | Production config scenario | todo |
| 11 | Reconcile the docs | partly done |
| 12 | Keep raw data | todo |

### 1. More rounds and spread

Use 5 or more rounds and report min/max or the standard deviation next to the median. Only call a gap real if it is
larger than the noise. The 3-round run of November 2026 shows why: inkgate's first round was 15 to 20% below its other
two in passthrough, json and merge, and in `merge` one KrakenD round beat one inkgate round. Gaps under about 10% are
within that spread.

### 2. Pin CPUs

Give each container its own cores with `--cpuset-cpus` (gateway, nginx, `oha`) and avoid SMT sibling threads, so two
containers never share a physical core. `--cpus` is a time quota: it limits how much CPU a container uses, not which
cores, so today the gateway, nginx and `oha` can land on the same core.

### 3. GOMAXPROCS=2 on both gateways

Since Go 1.25, the runtime reads the cgroup CPU limit and sets `GOMAXPROCS` from it, so with `--cpus 2` both gateways
already run with `GOMAXPROCS=2` (inkgate is built with Go 1.27, KrakenD CE 2.13.11 with Go 1.26.8). Set
`-e GOMAXPROCS=2` anyway, so the report does not depend on a runtime default, and record it in `meta.json`.

### 4. Document the memory choice

Explain why each gateway gets 512m (both peak under 50 MB in every current scenario, so 512m never constrains them),
and add a low-memory case, for example 128m, where the garbage collector has to work. Run it with and without
`GOMEMLIMIT`.

### 5. Vary payload size

Add small, medium (about 10 KB) and large (about 1 MB) bodies. The current bodies are under 200 bytes, which hides
differences in allocation, garbage collection and memory use. The milestone baseline already shows how much size
matters: forwarding 100 KB runs at 7% of the 1 KB rate.

### 6. Vary concurrency

Test 32, 128 and 512 connections, not only 128. Low concurrency shows latency, high concurrency shows queueing and
the per-connection memory.

### 7. Report errors

Show the success rate and the count of non-2xx answers per run, and fail the run if either is nonzero.
Already there: the report shows a success rate, and requests aborted by the load generator at the end of a run are
not counted as failures. Each scenario's answer is checked once before it is measured. Missing: non-2xx counts in the
report, and failing the run on them.

### 8. Record the host state

Log the CPU governor (set it to `performance`), turbo on or off, and the container engine version in `meta.json`.
Close other applications during runs. Already recorded: the CPU model, thread count, memory, kernel, images, limits
and run settings.

### 9. Separate host for load and backend

Run `oha` and nginx on another machine, or at least on another CPU set (see 2), so the load generator and the backend
do not compete with the gateway.

### 10. Production config scenario

Add a scenario with logging, metrics, CORS and rate limiting on. The current config turns all of them off, so the
numbers are an upper bound. In the milestone baseline, turning metrics on alone costs 20%.

### 11. Reconcile the docs

Two setups exist and their numbers must not be mixed:

| Numbers | Setup |
|---|---|
| [milestones/README.md](../milestones/README.md) baseline | a separate harness outside this repository: inkgate alone, 1 CPU, 2 GB, 128 connections, median of 3 |
| [benchmark/](../../benchmark/README.md), root README | this repository: inkgate against KrakenD CE, 2 CPUs and 512m each, 128 connections, median of 3 |

Each page now names its setup. Still to do: run the milestone jobs (100 KB bodies, JWT, Lua, metrics) in
`benchmark/` too, so there is one harness.

### 12. Keep raw data

Commit the raw output with the exact command line and the git commit hash of the build. Today `benchmark/results/raw/`
is in `.gitignore` and `meta.json` records neither the command line nor the commit, so a published number cannot be
traced back to its run.
