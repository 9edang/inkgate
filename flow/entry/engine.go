// Package entry is stage 2 of the flow for inkgate: where traffic enters.
//
// It owns everything that applies to the whole service rather than one endpoint:
//
//	NewRunServer   how the listener starts: Go http-server plugins > CORS > OpenTelemetry > HTTP(S) server
//	NewMiddlewares the service-wide middleware, in the order a request meets them:
//	               secure headers > Lua (service level) > bot detector > gorilla/mux router
//
// After routing, each request goes to the handler built by stage 3 (github.com/9edang/inkgate/flow/request).
package entry

import (
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	mux "github.com/9edang/inkgate/engine/router/mux"
	botdetector "github.com/9edang/inkgate/feature/security/botdetector/mux"
	httpsecure "github.com/9edang/inkgate/feature/security/httpsecure/mux"
	lua "github.com/9edang/inkgate/feature/transform/lua/mux"
)

// NewMiddlewares returns the service-wide middleware. The mux router applies
// the list in order, each one wrapping the previous, so the LAST item is the
// first to see a request.
func NewMiddlewares(cfg config.ServiceConfig, logger logging.Logger) []mux.HandlerMiddleware {
	var mws []mux.HandlerMiddleware

	// 3. bot detector, service level
	if mw := botdetector.NewMiddleware(cfg.ExtraConfig, logger); mw != nil {
		mws = append(mws, mw)
	}

	// 2. Lua, service level
	mws = lua.RegisterMiddleware(logger, cfg.ExtraConfig, mux.PathParamExtractor, mws)

	// 1. secure headers and host checks
	if mw := httpsecure.NewSecureMw(cfg.ExtraConfig); mw != nil {
		mws = append(mws, mw)
		logger.Debug("[SERVICE: mux][HTTPsecure] Successfully loaded module")
	}

	return mws
}
