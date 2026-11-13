// Package endpoint is stage 4 of the flow: the endpoint pipeline. It turns one
// incoming request into calls to the endpoint's backends and builds the response.
package endpoint

import (
	"fmt"
	"slices"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	otelpipeline "github.com/9edang/inkgate/feature/telemetry/otel/pipeline"
	lua "github.com/9edang/inkgate/feature/transform/lua/proxy"
	cel "github.com/9edang/inkgate/feature/validation/cel"
	jsonschema "github.com/9edang/inkgate/feature/validation/jsonschema"
)

type step func(proxy.Factory) proxy.Factory

// NewFactory returns the function that builds the pipeline of one endpoint.
// backendFactory builds each backend call (stage 5, flow/backend).
func NewFactory(logger logging.Logger, backendFactory proxy.BackendFactory, metricCollector *metrics.Metrics) proxy.Factory {
	// The steps a request goes through, in order.
	steps := []step{
		func(next proxy.Factory) proxy.Factory { return otelpipeline.ProxyFactory(next) }, // 1. OpenTelemetry span for the whole pipeline
		func(next proxy.Factory) proxy.Factory { // 2. pipeline metrics
			return metricCollector.ProxyFactory("pipe", next)
		},
		func(next proxy.Factory) proxy.Factory { // 3. endpoint Lua scripts (proxy level)
			return lua.ProxyFactory(logger, next)
		},
		func(next proxy.Factory) proxy.Factory { // 4. CEL rules on request and response
			return cel.ProxyFactory(logger, next)
		},
		func(next proxy.Factory) proxy.Factory { // 5. validate the request body with JSON schema
			return jsonschema.ProxyFactory(logger, next)
		},
		proxy.NewShadowFactory, // 6. send a copy to shadow backends, ignore their answers
	}

	// 7. the core pipeline (engine/proxy): static data > Go req/resp modifier plugins >
	// call every backend concurrently > merge their responses (+ flatmap).
	f := proxy.NewDefaultFactory(backendFactory, logger)
	for _, step := range slices.Backward(steps) {
		f = step(f)
	}

	return proxy.FactoryFunc(func(cfg *config.EndpointConfig) (proxy.Proxy, error) {
		logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Building the proxy pipe", cfg.Endpoint))
		return f.New(cfg)
	})
}
