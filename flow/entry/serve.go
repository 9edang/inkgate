package entry

import (
	"errors"
	"fmt"
	"time"

	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	xml "github.com/9edang/inkgate/feature/transform/xml"
	"github.com/9edang/inkgate/flow/boot"
	"github.com/9edang/inkgate/flow/request"
)

// Router is the gorilla/mux HTTP side of inkgate, handed to boot.Main by cmd/inkgate.
var Router = boot.Router{
	Name:             "router",
	NewMetrics:       newMetrics,
	RegisterEncoders: registerRenders,
	TestRoutes:       testRoutes,
	Serve:            Serve,
}

// Serve builds stage 2 (entry) and stage 3 (request) with gorilla/mux on top of the
// endpoint pipeline from boot, then runs the server until ctx is done.
func Serve(ctx context.Context, s boot.Services) {
	handlerFactory := request.NewHandlerFactory(s.Logger, &metrics.HTTPMetrics{Metrics: s.Metrics}, s.TokenRejecter)
	if s.Metrics != nil && s.Metrics.Config != nil && !s.Metrics.Config.RouterDisabled {
		// streams and sockets of the streaming endpoints, under router.stream.<endpoint>.*
		router.RegisterStreamObserver(metrics.NewStreamMetrics(s.Metrics.Router))
	}

	routerFactory := router.NewFactory(router.Config{
		Engine:         router.NewEngine(s.Config),
		HealthPath:     router.HealthPath(s.Config),
		HealthHandler:  router.NewHealthHandler(s.AgentPing),
		Middlewares:    NewMiddlewares(s.Config, s.Logger),
		HandlerFactory: handlerFactory,
		ProxyFactory:   s.EndpointFactory,
		Logger:         s.Logger,
		DebugPattern:   router.DefaultDebugPattern,
		EchoPattern:    router.DefaultEchoPattern,
		RunServer:      NewRunServer(s.Logger),
	})

	s.Logger.Info("Starting the inkgate instance")
	routerFactory.NewWithContext(ctx).Run(s.Config)
}

// newMetrics creates the collector and serves /__stats on its own ServeMux.
func newMetrics(ctx context.Context, cfg config.ServiceConfig, l logging.Logger) *metrics.Metrics {
	return metrics.NewHTTPMetrics(ctx, cfg.ExtraConfig, l).Metrics
}

// registerRenders adds the XML output encoding.
func registerRenders() {
	router.RegisterRender(xml.Name, xml.Render)
}

// testRoutes registers every endpoint on a real gorilla router for one second,
// so `check --test-routes` catches invalid route patterns.
func testRoutes(cfg config.ServiceConfig) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok {
				err = errors.New(s)
				return
			}
			err = fmt.Errorf("%v", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	router.DefaultFactory(proxy.DefaultFactory(logging.NoOp), logging.NoOp).NewWithContext(ctx).Run(cfg)

	return nil
}
