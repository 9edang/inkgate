# Adding a feature

Example: a header `X-Request-Source` that must be present on some endpoints (a request-stage check).

```mermaid
flowchart LR
    A["1 pick the stage<br/>(when does it run?)"] --> B["2 pick the domain<br/>(what is it?)"]
    B --> C["3 logic<br/>feature/&lt;domain&gt;/&lt;name&gt;"]
    C --> D{"needs HTTP context?"}
    D -- "no (endpoint / backend stage)" --> E["one step in flow/endpoint or flow/backend"]
    D -- "yes (entry / request stage)" --> F["a mux adapter in feature/&lt;domain&gt;/&lt;name&gt;/mux<br/>+ one step in flow/entry or flow/request"]
    E & F --> G["4 alias + docs + tests"]
```

## 1. Pick the stage

| Runs on... | Stage | Where | Step type |
|---|---|---|---|
| every request, before routing | entry | `flow/entry` | `router.HandlerMiddleware`, or a `RunServerFunc` wrapper |
| requests to one endpoint, with HTTP context (headers, status codes) | request | `flow/request` | `func(router.HandlerFactory) router.HandlerFactory` (`engine/router/mux`) |
| the endpoint's data (request and merged response) | endpoint | `flow/endpoint` | `func(proxy.Factory) proxy.Factory` |
| one backend call | backend | `flow/backend` | `func(proxy.BackendFactory) proxy.BackendFactory` |
| once at startup | boot | `flow/boot` | a function called from `Run` |

Our header check needs the HTTP request and should answer 400, so it goes in **request**: the logic in a
router-agnostic package and a small mux adapter next to it.

## 2. Pick the domain

`auth`, `security`, `validation`, `traffic`, `transform`, `connector` or `telemetry`. A required header is
**validation**: `feature/validation/requireheader`.

## 3. Write the logic, then the adapter

Router-agnostic (`feature/validation/requireheader/requireheader.go`):

```go
package requireheader

import "github.com/9edang/inkgate/engine/config"

const Namespace = "inkgate/feature/validation/requireheader"

// HeaderFor returns the header the endpoint requires, or "" when the feature is off.
func HeaderFor(cfg *config.EndpointConfig) string {
	v, _ := cfg.ExtraConfig[Namespace].(map[string]interface{})
	name, _ := v["header"].(string)
	return name
}
```

mux adapter (`feature/validation/requireheader/mux/mux.go`):

```go
package mux

import (
	"net/http"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	router "github.com/9edang/inkgate/engine/router/mux"
	shared "github.com/9edang/inkgate/feature/validation/requireheader"
)

func HandlerFactory(next router.HandlerFactory) router.HandlerFactory {
	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		h := next(cfg, p)
		name := shared.HeaderFor(cfg)
		if name == "" {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(name) == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			h(w, r)
		}
	}
}
```

Rules every feature follows:
- **Logic in the feature package, translation in the adapter.** The adapter only reads the request and writes the
  response.
- **Read only your own namespace.** If it's absent, return `next` unchanged, so the feature costs nothing.
- **Parse config once,** in the factory, not per request.
- **Share request data through the flow context**: `context.From(ctx)` (or `context.From(r.Context())` in an
  adapter) to read the request ID, claims or params, and `Set`/`Get` for values another layer needs.
- Namespaces start with `inkgate/` in every inkgate build, so configs stay portable between routers.

## 4. Add one step

In `flow/request/request.go`, at the position it should run (for example right after JWT validation):

```go
		requireheader.HandlerFactory, // 6. required headers
```

(import it as `requireheader "github.com/9edang/inkgate/feature/validation/requireheader/mux"`) and renumber the step comments that
follow it.

## 5. Alias, docs, tests

- Optional short alias: add `"inkgate/feature/validation/requireheader": "validation/require-header"` to `aliases`
  in `flow/boot/main.go`.
- Add a row to [config-reference.md](config-reference.md), a section to the domain page in `docs/feature/`, and the
  step to `docs/flow/3-request.md`.
- Unit-test the adapter with `httptest`; add an integration spec in `tests/fixtures/specs/`.
- Run `make check-layout` to confirm the feature package doesn't import the router.
