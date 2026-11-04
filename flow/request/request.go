// Package request is stage 3 of the flow for inkgate: what happens to one
// HTTP request after the router picked its endpoint and before the endpoint
// pipeline runs.
package request

import (
	"fmt"
	"net/http"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	mux "github.com/9edang/inkgate/engine/router/mux"
	"github.com/9edang/inkgate/engine/transport/http/server"
	jose "github.com/9edang/inkgate/feature/auth/jose"
	muxjose "github.com/9edang/inkgate/feature/auth/jose/mux"
	botdetector "github.com/9edang/inkgate/feature/security/botdetector/mux"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics/mux"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus/mux"
	otelmux "github.com/9edang/inkgate/feature/telemetry/otel/mux"
	ratelimit "github.com/9edang/inkgate/feature/traffic/ratelimit/mux"
	lua "github.com/9edang/inkgate/feature/transform/lua/mux"
)

type step func(mux.HandlerFactory) mux.HandlerFactory

// NewHandlerFactory returns the function that builds the HTTP handler of one endpoint.
// rejecter decides whether a valid JWT has been revoked (built at boot).
func NewHandlerFactory(logger logging.Logger, metricCollector *metrics.Metrics, rejecter jose.RejecterFactory) mux.HandlerFactory {
	params := mux.PathParamExtractor

	// The steps a request goes through, in order. Each step either rejects the
	// request or passes it to the next one.
	steps := []step{
		otelmux.New, // 1. start the OpenTelemetry span
		func(next mux.HandlerFactory) mux.HandlerFactory { // 2. block known bots
			return botdetector.New(next, logger)
		},
		opencensus.New,                        // 3. OpenCensus span (legacy tracing)
		metricCollector.NewHTTPHandlerFactory, // 4. count the request
		func(next mux.HandlerFactory) mux.HandlerFactory { // 5. validate the JWT, check revocation
			return muxjose.HandlerFactory(next, params, logger, rejecter)
		},
		func(next mux.HandlerFactory) mux.HandlerFactory { // 6. run endpoint Lua scripts
			return lua.HandlerFactory(logger, next, params)
		},
		func(next mux.HandlerFactory) mux.HandlerFactory { // 7. endpoint rate limit
			return ratelimit.NewRateLimiterMw(logger, next)
		},
	}

	// 8. turn the request (with its path params) into a proxy.Request, call stage 4
	// (flow/endpoint) and write the response with the endpoint's output encoding.
	h := mux.CustomEndpointHandlerWithHTTPError(mux.NewRequestWithPathParams, server.DefaultToHTTPError)
	for i := len(steps) - 1; i >= 0; i-- {
		h = steps[i](h)
	}

	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Building the http handler", cfg.Endpoint))
		return h(cfg, p)
	}
}
