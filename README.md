# inkgate

inkgate is an API gateway. It receives an HTTP request, applies security and traffic rules, calls one or more
backends, merges their answers and returns one response. This repository builds it on the
[gorilla/mux](https://github.com/gorilla/mux) router.

Inspired by [KrakenD](https://www.krakend.io) ([krakend-ce](https://github.com/krakend/krakend-ce)) and the
[Lura](https://luraproject.org) framework it is built on: inkgate keeps their configuration format and most of their
feature set, and adds what they leave out (Server-Sent Events and WebSocket proxying, a leaner request path). Their
copyright notices are kept in [NOTICE](NOTICE), as the Apache-2.0 license requires.

## Get it

```sh
git clone https://github.com/9edang/inkgate.git
cd inkgate
go install github.com/9edang/inkgate/cmd/inkgate@latest    # or build it here, below
```

## Quick start

```sh
make build                              # bin/inkgate (VERSION=0.1.0 by default)
bin/inkgate check -c inkgate.json       # validate a config
bin/inkgate run -c inkgate.json -d      # run (-d enables /__debug and debug logs)
bin/inkgate --help                      # run, check, check-plugin, audit, test-plugin, version
```

Flexible config (templates) is enabled with `FC_ENABLE=1` and `FC_SETTINGS`, `FC_PARTIALS`, `FC_TEMPLATES`,
`FC_OUT`.

## Layout

```
inkgate/
├── cmd/inkgate/     the binary: boot.Main(entry.Router)
├── context/         the flow context: core data of one request, carried by every layer (replaces "context")
├── flow/            WHEN things happen: boot → entry → request → endpoint → backend
├── feature/         WHAT the gateway can do; router code sits next to its feature
├── engine/          the plumbing: config, proxy pipeline, HTTP transport, discovery, CLI; the router in engine/router
├── internal/        helper libraries
├── examples/        small runnable programs (the Lua binder examples)
├── schema/          the JSON schema of inkgate.json, embedded for `check --lint-no-network`
├── docs/            documentation, and the milestones (what to fix, speed up and add next)
└── tests/           integration suite, run against bin/inkgate
```

Start reading at [docs/README.md](docs/README.md). What comes next is in [docs/milestones](docs/milestones/README.md).

## Benchmark

<!-- benchmark:start -->
inkgate against KrakenD CE `2.13.11`, in containers with the same config and the same limits (2 CPUs and 512m memory each, 128 connections, 30s per run after 30s of warm-up, 7 rounds: the median is shown, with the lowest to highest round in brackets), on AMD Ryzen 5 5600 6-Core Processor. "Within noise" means the ranges of the two gateways overlap, so the lead is not a result.

| Scenario | inkgate req/s (range) | KrakenD CE req/s (range) | inkgate vs KrakenD CE | inkgate p99 | KrakenD CE p99 | inkgate memory | KrakenD CE memory |
|---|--:|--:|--:|--:|--:|--:|--:|
| passthrough | **32,469** (32,310 to 32,536) | 23,402 (23,300 to 23,551) | 1.39x | 10.2 ms | 12.7 ms | 39 MB | 40 MB |
| json | **30,390** (30,132 to 30,668) | 27,496 (27,454 to 27,611) | 1.11x | 10.8 ms | 11.8 ms | 38 MB | 43 MB |
| merge | **17,242** (17,219 to 17,349) | 15,897 (15,867 to 16,125) | 1.08x | 15.3 ms | 16.6 ms | 45 MB | 45 MB |
| routes | **24,712** (24,474 to 24,923) | 26,897 (26,664 to 27,124) | 0.92x | 13.5 ms | 12.1 ms | 48 MB | 43 MB |
| plugin-modifier | **30,151** (29,861 to 30,193) | 28,074 (27,766 to 28,324) | 1.07x | 11.1 ms | 11.5 ms | 44 MB | 42 MB |
| plugin-server | **29,880** (29,832 to 30,079) | 27,117 (26,288 to 27,304) | 1.10x | 11.1 ms | 11.9 ms | 44 MB | 42 MB |
| plugin-client | **21,025** (20,998 to 21,226) | 19,998 (19,943 to 20,165) | 1.05x | 14.2 ms | 14.9 ms | 45 MB | 42 MB |

Go: inkgate 1.27.4, KrakenD CE 1.26.8. They are different, so the comparison includes the Go toolchain and runtime, not only the gateway code.

![requests per second](benchmark/results/rps.svg)

Setup, scenarios and how to rerun it: [benchmark/](benchmark/README.md). Full report: [benchmark/results/report.md](benchmark/results/report.md).
<!-- benchmark:end -->

## Tests

```sh
make test          # build, unit tests and the integration suite
```

Known unit-test failures and their causes: [docs/known-issues.md](docs/known-issues.md).

## License

Apache-2.0, Copyright 2026 9edang. See [LICENSE](LICENSE) and [NOTICE](NOTICE). The code derives from KrakenD and Lura,
also Apache-2.0; their attribution notices are reproduced in NOTICE and must stay with any copy or fork.
