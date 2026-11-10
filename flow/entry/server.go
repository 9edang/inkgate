package entry

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	corerouter "github.com/9edang/inkgate/engine/router"
	serverhttp "github.com/9edang/inkgate/engine/transport/http/server"
	serverplugin "github.com/9edang/inkgate/engine/transport/http/server/plugin"
	cors "github.com/9edang/inkgate/feature/security/cors"
	otelpipeline "github.com/9edang/inkgate/feature/telemetry/otel/pipeline"
	"net/http"
)

// NewRunServer returns the function that starts the HTTP listener.
//
// Each wrapper wraps the handler it receives and passes it on, so the wrapper
// added first sits closest to the server and runs first. Request order:
//  0. flow context: the request's *context.Flow is created before anything else
//  1. OpenTelemetry server instrumentation
//  2. CORS (answers preflight requests before routing)
//  3. Go http-server plugins (`plugin/http-server` in the config), right before the router
//  4. the router
//
// The HTTP(S) server itself (TLS, timeouts, graceful shutdown) is
// serverhttp.RunServerWithLoggerFactory.
func NewRunServer(logger logging.Logger) corerouter.RunServerFunc {
	run := serverhttp.RunServerWithLoggerFactory(logger)
	run = withFlowContext(run, logger)
	run = otelpipeline.GlobalRunServer(logger, run)
	run = cors.NewRunServerWithLogger(run, logger)
	return corerouter.RunServerFunc(serverplugin.New(logger, serverplugin.RunServer(run)))
}

// withFlowContext wraps the whole handler so every request starts with its flow
// context, and logs a one-line summary when the response has been written.
func withFlowContext(next corerouter.RunServerFunc, logger logging.Logger) corerouter.RunServerFunc {
	return func(ctx context.Context, cfg config.ServiceConfig, handler http.Handler) error {
		mw := context.Middleware(context.ParseOptions(cfg.ExtraConfig))
		debug := logging.Enabled(logger, logging.LEVEL_DEBUG)
		summary := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
			if !debug {
				return // the line would be dropped: do not build it
			}
			c := context.From(r.Context())
			endpoint, method := c.Endpoint()
			logger.Debug(fmt.Sprintf("[FLOW][%s] %s %s endpoint=%q method=%s backends=%d elapsed=%s",
				c.RequestID(), r.Method, r.URL.Path, endpoint, method, len(c.BackendCalls()), c.Elapsed()))
		})
		return next(ctx, cfg, mw(summary))
	}
}
