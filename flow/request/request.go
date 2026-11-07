// Package request is stage 3 of the flow for inkgate: what happens to one
// HTTP request after the router picked its endpoint and before the endpoint
// pipeline runs.
package request

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
	"github.com/9edang/inkgate/engine/transport/http/server"
	jose "github.com/9edang/inkgate/feature/auth/jose"
	"github.com/9edang/inkgate/feature/security/botdetector/gateway"
	"github.com/9edang/inkgate/feature/telemetry/metrics"
	"github.com/9edang/inkgate/feature/telemetry/opencensus"
	kotel "github.com/9edang/inkgate/feature/telemetry/otel"
	ratelimitrouter "github.com/9edang/inkgate/feature/traffic/ratelimit/router"
	luarouter "github.com/9edang/inkgate/feature/transform/lua/router"
)

type step func(router.HandlerFactory) router.HandlerFactory

// NewHandlerFactory returns the function that builds the HTTP handler of one endpoint.
// rejecter decides whether a valid JWT has been revoked (built at boot).
func NewHandlerFactory(logger logging.Logger, metricCollector *metrics.HTTPMetrics, rejecter jose.RejecterFactory) router.HandlerFactory {
	params := router.PathParamExtractor

	// The steps a request goes through, in order. Each step either rejects the
	// request or passes it to the next one.
	steps := []step{
		kotel.New, // 1. start the OpenTelemetry span
		func(next router.HandlerFactory) router.HandlerFactory { // 2. block known bots
			return gateway.New(next, logger)
		},
		opencensus.New,                        // 3. OpenCensus span (legacy tracing)
		metricCollector.NewHTTPHandlerFactory, // 4. count the request
		func(next router.HandlerFactory) router.HandlerFactory { // 5. validate the JWT, check revocation
			return jose.HandlerFactory(next, params, logger, rejecter)
		},
		func(next router.HandlerFactory) router.HandlerFactory { // 6. run endpoint Lua scripts
			return luarouter.HandlerFactory(logger, next, params)
		},
		func(next router.HandlerFactory) router.HandlerFactory { // 7. endpoint rate limit
			return ratelimitrouter.NewRateLimiterMw(logger, next)
		},
	}

	// 8. turn the request (with its path params) into a proxy.Request, call stage 4
	// (flow/endpoint) and write the response with the endpoint's output encoding.
	h := router.CustomEndpointHandlerWithHTTPError(router.NewRequestWithPathParams, server.DefaultToHTTPError)
	for _, step := range slices.Backward(steps) {
		h = step(h)
	}

	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Building the http handler", cfg.Endpoint))
		return h(cfg, p)
	}
}
