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
git clone git@github.com:9edang/inkgate.git
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
├── feature/         WHAT the gateway can do; mux adapters sit next to their feature in feature/<domain>/<name>/mux
├── engine/          the plumbing: config, proxy pipeline, HTTP transport, discovery, CLI; the router in engine/router/mux
├── internal/        helper libraries
├── docs/            documentation, and the milestones (what to fix, speed up and add next)
└── tests/           integration suite, run against bin/inkgate
```

Start reading at [docs/README.md](docs/README.md). What comes next is in [docs/milestones](docs/milestones/README.md).

## Tests

```sh
make test          # build, unit tests and the integration suite
make check-layout  # gorilla/mux code only in the router, the adapters and the HTTP stages
```

Known unit-test failures and their causes: [docs/known-issues.md](docs/known-issues.md).

## License

Apache-2.0, Copyright 2026 9edang. See [LICENSE](LICENSE) and [NOTICE](NOTICE). The code derives from KrakenD and Lura,
also Apache-2.0; their attribution notices are reproduced in NOTICE and must stay with any copy or fork.
