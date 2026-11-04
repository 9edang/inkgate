package entry

import (
	"errors"
	"fmt"
	"github.com/9edang/inkgate/context"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	mux "github.com/9edang/inkgate/engine/router/mux"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	muxmetrics "github.com/9edang/inkgate/feature/telemetry/metrics/mux"
	xml "github.com/9edang/inkgate/feature/transform/xml"
	muxxml "github.com/9edang/inkgate/feature/transform/xml/mux"
	"github.com/9edang/inkgate/flow/boot"
	"github.com/9edang/inkgate/flow/request"
)

// Router is the gorilla/mux HTTP side of inkgate, handed to boot.Main by cmd/inkgate.
var Router = boot.Router{
	Name:             "mux",
	NewMetrics:       newMetrics,
	RegisterEncoders: registerRenders,
	TestRoutes:       testRoutes,
	Serve:            Serve,
}

// Serve builds stage 2 (entry) and stage 3 (request) with gorilla/mux on top of the
// endpoint pipeline from boot, then runs the server until ctx is done.
func Serve(ctx context.Context, s boot.Services) {
	handlerFactory := request.NewHandlerFactory(s.Logger, &muxmetrics.Metrics{Metrics: s.Metrics}, s.TokenRejecter)

	routerFactory := mux.NewFactory(mux.Config{
		Engine:         mux.NewEngine(s.Config),
		HealthPath:     mux.HealthPath(s.Config),
		HealthHandler:  mux.NewHealthHandler(s.AgentPing),
		Middlewares:    NewMiddlewares(s.Config, s.Logger),
		HandlerFactory: handlerFactory,
		ProxyFactory:   s.EndpointFactory,
		Logger:         s.Logger,
		DebugPattern:   mux.DefaultDebugPattern,
		EchoPattern:    mux.DefaultEchoPattern,
		RunServer:      mux.RunServerFunc(NewRunServer(s.Logger)),
	})

	s.Logger.Info("Starting the inkgate instance")
	routerFactory.NewWithContext(ctx).Run(s.Config)
}

// newMetrics creates the collector and serves /__stats on its own ServeMux.
func newMetrics(ctx context.Context, cfg config.ServiceConfig, l logging.Logger) *metrics.Metrics {
	return muxmetrics.New(ctx, cfg.ExtraConfig, l).Metrics
}

// registerRenders adds the XML output encoding.
func registerRenders() {
	mux.RegisterRender(xml.Name, muxxml.Render)
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
	mux.DefaultFactory(proxy.DefaultFactory(logging.NoOp), logging.NoOp).NewWithContext(ctx).Run(cfg)
	return nil
}
