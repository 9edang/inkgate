# engine — gateway machinery

The generic gateway. It knows no feature and no router: config model and parsing, the core proxy pipeline (merge,
static, shadow, concurrency, load balancing), HTTP client and server, encodings, service discovery, async agents,
Go plugin loading and the CLI framework.

Features plug into it through `engine/proxy` (`Factory`, `BackendFactory`). Routers implement `engine/router` (`Factory`, `RunServerFunc`, the router `Namespace`); the gorilla/mux
implementation is `engine/router`.
