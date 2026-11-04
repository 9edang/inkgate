// Package backend is stage 5 of the flow: one call from inkgate to one backend.
package backend

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/client"
	clientplugin "github.com/9edang/inkgate/engine/transport/http/client/plugin"
	"github.com/9edang/inkgate/engine/transport/http/socket"
	oauth2client "github.com/9edang/inkgate/feature/auth/oauth2client"
	amqp "github.com/9edang/inkgate/feature/connector/amqp"
	lambda "github.com/9edang/inkgate/feature/connector/lambda"
	pubsub "github.com/9edang/inkgate/feature/connector/pubsub"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	otelpipeline "github.com/9edang/inkgate/feature/telemetry/otel/pipeline"
	cb "github.com/9edang/inkgate/feature/traffic/circuitbreaker/gobreaker/proxy"
	httpcache "github.com/9edang/inkgate/feature/traffic/httpcache"
	ratelimit "github.com/9edang/inkgate/feature/traffic/ratelimit/proxy"
	lua "github.com/9edang/inkgate/feature/transform/lua/proxy"
	martian "github.com/9edang/inkgate/feature/transform/martian"
	cel "github.com/9edang/inkgate/feature/validation/cel"
	"slices"
)

type step func(proxy.BackendFactory) proxy.BackendFactory

// NewFactory returns the function that builds the call to one backend.
//
// Before these steps run, the core pipeline (engine/proxy) already filtered the
// query string and headers, built the backend URL, picked a host (load balancing)
// and fanned out concurrent calls.
func NewFactory(ctx context.Context, logger logging.Logger, metricCollector *metrics.Metrics) proxy.BackendFactory {
	// The steps a backend call goes through, in order.
	steps := []step{
		recordFlow,                  // 0. record the call in the flow context (+ optional request ID header)
		otelpipeline.BackendFactory, // 1. OpenTelemetry span for this backend
		opencensus.BackendFactory,   // 2. OpenCensus span (legacy tracing)
		func(next proxy.BackendFactory) proxy.BackendFactory { // 3. backend metrics
			return metricCollector.BackendFactory("backend", next)
		},
		func(next proxy.BackendFactory) proxy.BackendFactory { // 4. circuit breaker
			return cb.BackendFactory(next, logger)
		},
		func(next proxy.BackendFactory) proxy.BackendFactory { // 5. backend rate limit
			return ratelimit.BackendFactory(logger, next)
		},
		func(next proxy.BackendFactory) proxy.BackendFactory { // 6. backend Lua scripts
			return lua.BackendFactory(logger, next)
		},
		func(next proxy.BackendFactory) proxy.BackendFactory { // 7. backend CEL rules
			return cel.BackendFactory(logger, next)
		},
		// 8-10. non-HTTP backends: each one answers the call itself when the
		// backend is configured for it, otherwise passes it down.
		func(next proxy.BackendFactory) proxy.BackendFactory {
			return lambda.BackendFactory(logger, next)
		},
		func(next proxy.BackendFactory) proxy.BackendFactory {
			return amqp.NewBackendFactory(ctx, logger, next)
		},
		func(next proxy.BackendFactory) proxy.BackendFactory {
			return pubsub.NewBackendFactory(ctx, logger, next).New
		},
		socket.BackendFactory, // 11. WebSocket endpoints (router/stream mode socket): proxy the socket instead of one HTTP call
	}

	// 12. the HTTP call: Martian request/response modifiers around the HTTP client.
	f := martian.NewConfiguredBackendFactory(logger, newHTTPClient(ctx, logger))
	for _, step := range slices.Backward(steps) {
		f = step(f)
	}

	return func(remote *config.Backend) proxy.Proxy {
		logger.Debug(fmt.Sprintf("[BACKEND: %s] Building the backend pipe", remote.URLPattern))
		return f(remote)
	}
}

// newHTTPClient builds the HTTP client of a backend. Outermost first:
// Go http-client plugins > OpenCensus > OpenTelemetry > HTTP cache > OAuth2
// client credentials (when configured) > net/http client.
func newHTTPClient(ctx context.Context, logger logging.Logger) func(*config.Backend) client.HTTPRequestExecutor {
	return clientplugin.HTTPRequestExecutorWithContext(ctx, logger,
		func(cfg *config.Backend) client.HTTPRequestExecutor {
			newClient := client.NewHTTPClient
			if _, ok := cfg.ExtraConfig[oauth2client.Namespace]; ok {
				newClient = oauth2client.NewHTTPClient(cfg)
			}
			newClient = httpcache.NewHTTPClient(cfg, newClient)
			newClient = otelpipeline.InstrumentedHTTPClientFactory(newClient, cfg)
			return opencensus.HTTPRequestExecutorFromConfig(newClient, cfg)
		})
}
