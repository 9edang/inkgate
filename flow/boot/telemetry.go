package boot

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/env"
	"net/http"
	"time"

	"github.com/go-contrib/uuid"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/core"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/sd/dnssrv"
	serverhttp "github.com/9edang/inkgate/engine/transport/http/server"
	revoker "github.com/9edang/inkgate/feature/auth/bloomfilter/gateway"
	jose "github.com/9edang/inkgate/feature/auth/jose"
	audit "github.com/9edang/inkgate/feature/telemetry/audit"
	influxdb "github.com/9edang/inkgate/feature/telemetry/influx"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	kotel "github.com/9edang/inkgate/feature/telemetry/otel"
	usage "github.com/9edang/inkgate/feature/telemetry/usage"
	cel "github.com/9edang/inkgate/feature/validation/cel"
)

// registerTelemetry starts InfluxDB, OpenCensus and OpenTelemetry as the config
// requires. The metrics collector itself comes from the router variant.
// The returned func releases them on shutdown.
func registerTelemetry(ctx context.Context, cfg config.ServiceConfig, l logging.Logger, metricCollector *metrics.Metrics) func() {
	if err := influxdb.New(ctx, cfg.ExtraConfig, metricCollector, l); err != nil {
		if err != influxdb.ErrNoConfig {
			l.Warning("[SERVICE: InfluxDB]", err.Error())
		}
	} else {
		l.Debug("[SERVICE: InfluxDB] Service correctly registered")
	}

	if err := opencensus.Register(ctx, cfg, opencensus.DefaultViews...); err != nil {
		if err != opencensus.ErrNoConfig {
			l.Warning("[SERVICE: OpenCensus]", err.Error())
		}
	} else {
		l.Debug("[SERVICE: OpenCensus] Service correctly registered")
	}

	shutdown := func() {}
	if fn, err := kotel.Register(ctx, l, cfg); err == nil {
		shutdown = fn
	} else {
		l.Error(fmt.Sprintf("[SERVICE: OpenTelemetry] cannot register exporters: %s", err.Error()))
	}
	return shutdown
}

// newTokenRejecter sets up the JWK cache and the JWT revocation checks
// (bloom filter and CEL rules).
func newTokenRejecter(ctx context.Context, cfg config.ServiceConfig, l logging.Logger) jose.ChainedRejecterFactory {
	if err := jose.SetGlobalCacher(l, cfg.ExtraConfig); err != nil && err != jose.ErrNoValidatorCfg {
		l.Error("[SERVICE: JOSE]", err.Error())
	}

	dnssrv.Register()
	registerService := func(name string, port int) {}

	rejecter, err := revoker.Register(ctx, "inkgate-bf", cfg, l, registerService)
	if err != nil && err != revoker.ErrNoConfig {
		l.Warning("[SERVICE: Bloomfilter]", err.Error())
	}

	return jose.ChainedRejecterFactory([]jose.RejecterFactory{
		jose.RejecterFactoryFunc(func(_ logging.Logger, _ *config.EndpointConfig) jose.Rejecter {
			return jose.RejecterFunc(rejecter.RejectToken)
		}),
		jose.RejecterFactoryFunc(func(l logging.Logger, cfg *config.EndpointConfig) jose.Rejecter {
			if r := cel.NewRejecter(l, cfg); r != nil {
				return r
			}
			return jose.FixedRejecter(false)
		}),
	})
}

const (
	usageDelay = 5 * time.Second
)

// startUsageReporter sends anonymous usage stats to INKGATE_USAGE_URL. Off when the variable is unset.
func startUsageReporter(ctx context.Context, logger logging.Logger, cfg config.ServiceConfig) {
	logPrefix := "[SERVICE: Telemetry]"
	reportURL := env.App().UsageURL
	if reportURL == "" {
		return
	}

	clusterID, err := cfg.Hash()
	if err != nil {
		logger.Debug(logPrefix, "Unable to create the Cluster ID hash:", err.Error())
		return
	}

	go func() {
		time.Sleep(usageDelay)

		serverID := uuid.NewV4().String()
		logger.Debug(logPrefix, "Registering usage stats for Cluster ID", clusterID)

		s := audit.Parse(&cfg)
		a, _ := audit.Marshal(&s)
		if err := usage.Report(
			ctx,
			usage.Options{
				URL:          reportURL,
				ClusterID:    clusterID,
				ServerID:     serverID,
				Version:      core.InkgateVersion,
				UserAgent:    core.InkgateUserAgent,
				ExtraPayload: a,
				Client:       &http.Client{Transport: serverhttp.NewTransport(cfg, logger)},
			},
			nil,
		); err != nil {
			logger.Debug(logPrefix, "Unable to create the usage report client:", err.Error())
		}
	}()
}
