// Package boot is stage 1 of the flow: starting inkgate.
//
// Main reads the command line and config, then Run follows the startup
// checklist. Run prepares the shared services (logging, telemetry, plugins,
// JWT revocation), builds the router-independent stages
//
//	endpoint  pipeline (validation, merge, ...)
//	backend   one backend call (QoS, connectors, HTTP client)
//
// and hands them to entry.Router.Serve, which builds the HTTP stages
// (entry, request) with net/http.
package boot

import (
	"fmt"
	"os"

	"github.com/9edang/inkgate/context"

	"golang.org/x/sync/errgroup"

	"github.com/9edang/inkgate/engine/async"
	cmd "github.com/9edang/inkgate/engine/cli"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/core"
	"github.com/9edang/inkgate/engine/sd/dnssrv"
	asyncamqp "github.com/9edang/inkgate/feature/connector/amqp/async"
	"github.com/9edang/inkgate/flow/backend"
	"github.com/9edang/inkgate/flow/endpoint"
)

// NewExecutor returns the function the CLI calls to start the gateway.
func NewExecutor(ctx context.Context, r Router) cmd.Executor {
	return func(cfg config.ServiceConfig) { Run(ctx, cfg, r) }
}

// Run starts the gateway and blocks until ctx is cancelled or the server stops.
func Run(ctx context.Context, cfg config.ServiceConfig, r Router) {
	cfg.Normalize()

	// 1. logging
	logger, gelfWriter, err := newLogger(cfg)
	if err != nil {
		return
	}
	logger.Info("Starting inkgate v" + core.InkgateVersion)
	startUsageReporter(ctx, logger, cfg)
	if wd, err := os.Getwd(); err == nil {
		logger.Info("Working directory is", wd)
	}

	// 2. global settings and Go .so plugins
	dnssrv.SetTTL(cfg.DNSCacheTTL)
	if cfg.Plugin != nil {
		LoadPluginsWithContext(ctx, cfg.Plugin.Folder, cfg.Plugin.Pattern, logger)
	}

	// 3. shared services: metrics, tracing, JWT revocation
	metricCollector := r.NewMetrics(ctx, cfg, logger)
	closeTelemetry := registerTelemetry(ctx, cfg, logger, metricCollector)
	defer closeTelemetry()
	tokenRejecter := newTokenRejecter(ctx, cfg, logger)

	// 4. the router-independent stages: backend, then endpoint on top of it
	backendFactory := backend.NewFactory(ctx, logger, metricCollector)
	endpointFactory := endpoint.NewFactory(logger, backendFactory, metricCollector)

	agentPing := make(chan string, len(cfg.AsyncAgents))
	services := Services{
		Config:          cfg,
		Logger:          logger,
		LogWriter:       gelfWriter,
		Metrics:         metricCollector,
		TokenRejecter:   tokenRejecter,
		EndpointFactory: endpointFactory,
		AgentPing:       agentPing,
	}

	// 5. serve HTTP through the variant, plus the async agents (AMQP consumers) when configured
	if len(cfg.AsyncAgents) == 0 {
		r.Serve(ctx, services)
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	gctx, closeGroupCtx := context.WithCancel(gctx)
	agents := async.AgentStarter([]async.Factory{asyncamqp.StartAgent})

	if cfg.SequentialStart {
		g.Go(agents.Start(gctx, cfg.AsyncAgents, logger, (chan<- string)(agentPing), endpointFactory))
	} else {
		g.Go(func() error {
			return agents.Start(gctx, cfg.AsyncAgents, logger, (chan<- string)(agentPing), endpointFactory)()
		})
	}

	g.Go(func() error {
		logger.Info(fmt.Sprintf("[SERVICE: %s] Building the router", r.Name))
		r.Serve(ctx, services)
		closeGroupCtx()

		return nil
	})

	// #nosec G104 -- error is not actionable here
	_ = g.Wait()
}
