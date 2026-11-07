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
	"github.com/9edang/inkgate/engine/router"
	"github.com/9edang/inkgate/feature/security/botdetector/gateway"
	"github.com/9edang/inkgate/feature/security/httpsecure"
	luarouter "github.com/9edang/inkgate/feature/transform/lua/router"
)

// NewMiddlewares returns the service-wide middleware. The router applies
// the list in order, each one wrapping the previous, so the LAST item is the
// first to see a request.
func NewMiddlewares(cfg config.ServiceConfig, logger logging.Logger) []router.HandlerMiddleware {
	var mws []router.HandlerMiddleware

	// 3. bot detector, service level
	if mw := gateway.NewMiddleware(cfg.ExtraConfig, logger); mw != nil {
		mws = append(mws, mw)
	}

	// 2. Lua, service level
	mws = luarouter.RegisterMiddleware(logger, cfg.ExtraConfig, router.PathParamExtractor, mws)

	// 1. secure headers and host checks
	if mw := httpsecure.NewSecureMw(cfg.ExtraConfig); mw != nil {
		mws = append(mws, mw)
		logger.Debug("[SERVICE: Router][HTTPsecure] Successfully loaded module")
	}

	return mws
}
