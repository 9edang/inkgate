# inkgate documentation

inkgate is an API gateway: it receives an HTTP request, runs it through security and traffic rules, calls one or
more backends, merges their answers and returns a single response. One Go module, organized the way a request
travels, built as the **inkgate** binary on the [gorilla/mux](https://github.com/gorilla/mux) router.

Diagrams are [Mermaid](https://mermaid.js.org); they render on GitHub, GitLab and in VS Code's Markdown preview.

## Reading paths

| If you are... | Read |
|---|---|
| new to the project | [overview/architecture.md](overview/architecture.md) → [overview/request-lifecycle.md](overview/request-lifecycle.md) → [overview/flow-context.md](overview/flow-context.md) → [overview/startup.md](overview/startup.md) |
| working on the HTTP side (gorilla/mux) | [engine/gateway.md](engine/gateway.md) |
| debugging a request | [flow/](flow) stage by stage, starting at [flow/2-entry.md](flow/2-entry.md) |
| changing a capability (JWT, rate limit, Lua...) | the matching page in [feature/](feature) |
| touching config, the core pipeline or HTTP transport | [engine/](engine) |
| adding something | [guides/add-a-feature.md](guides/add-a-feature.md) |
| writing a custom plugin (.so) | [guides/custom-plugins.md](guides/custom-plugins.md) |
| writing an `inkgate.json` | [guides/config-reference.md](guides/config-reference.md) |
| planning the next work | [milestones/](milestones/README.md) |
| benchmarking against KrakenD CE | [guides/benchmarking.md](guides/benchmarking.md) |
| getting ready for production | [production-readiness.md](production-readiness.md) |
| following Lura and KrakenD: fork baseline, security fixes, removal candidates | [upstream.md](upstream.md) |
| looking for performance and resource risks | [performance-review.md](performance-review.md) |

## Map

```
docs/
├── overview/            the big picture
│   ├── architecture.md      layers (flow / feature / engine) and folder map
│   ├── request-lifecycle.md one request end to end, sequence diagram
│   ├── flow-context.md      the flow context: the data every layer shares for one request
│   └── startup.md           from the command line to the first request
├── flow/                one page per stage
│   ├── 1-boot.md   2-entry.md   3-request.md   4-endpoint.md   5-backend.md
├── engine/              shared machinery
│   ├── config.md  gateway.md  router.md  proxy.md  transport.md  encoding-and-discovery.md  async-and-plugins.md  cli.md
├── feature/             one page per business domain
│   ├── auth.md  security.md  validation.md  traffic.md  transform.md  connector.md  telemetry.md
├── guides/              how-to
│   ├── add-a-feature.md  config-reference.md  custom-plugins.md  testing.md  benchmarking.md
├── milestones/          what to fix, speed up and add next, with the evidence for each item
│   ├── README.md  m1-confirmed-bugs.md  m2-request-cost.md  m3-streaming.md  m4-traffic-resilience.md  m5-operability.md
├── production-readiness.md  checklist before serving production traffic
├── upstream.md          what inkgate was forked from and how to follow it
├── performance-review.md    request-path review: findings, status and order of work
├── design/              proposals under review (json-passthrough.md)
└── known-issues.md      unit tests that fail and why
```
