package boot

import (
	"errors"

	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/sd/dnssrv"
	revoker "github.com/9edang/inkgate/feature/auth/bloomfilter/gateway"
	jose "github.com/9edang/inkgate/feature/auth/jose"
	influxdb "github.com/9edang/inkgate/feature/telemetry/influx"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	kotel "github.com/9edang/inkgate/feature/telemetry/otel"
	cel "github.com/9edang/inkgate/feature/validation/cel"
)

// registerTelemetry starts InfluxDB, OpenCensus and OpenTelemetry as the config
// requires. The metrics collector itself comes from the router variant.
// The returned func releases them on shutdown.
func registerTelemetry(ctx context.Context, cfg config.ServiceConfig, l logging.Logger, metricCollector *metrics.Metrics) func() {
	if err := influxdb.New(ctx, cfg.ExtraConfig, metricCollector, l); err != nil {
		if !errors.Is(err, influxdb.ErrNoConfig) {
			l.Warning("[SERVICE: InfluxDB]", err.Error())
		}
	} else {
		l.Debug("[SERVICE: InfluxDB] Service correctly registered")
	}

	if err := opencensus.Register(ctx, cfg, opencensus.DefaultViews...); err != nil {
		if !errors.Is(err, opencensus.ErrNoConfig) {
			l.Warning("[SERVICE: OpenCensus]", err.Error())
		}
	} else {
		l.Debug("[SERVICE: OpenCensus] Service correctly registered")
	}

	shutdown := func() {}
	if fn, err := kotel.Register(ctx, l, cfg); err == nil {
		shutdown = fn
	} else {
		l.Error("[SERVICE: OpenTelemetry] cannot register exporters: " + err.Error())
	}

	return shutdown
}

// newTokenRejecter sets up the JWK cache and the JWT revocation checks
// (bloom filter and CEL rules).
func newTokenRejecter(ctx context.Context, cfg config.ServiceConfig, l logging.Logger) jose.ChainedRejecterFactory {
	if err := jose.SetGlobalCacher(l, cfg.ExtraConfig); err != nil && !errors.Is(err, jose.ErrNoValidatorCfg) {
		l.Error("[SERVICE: JOSE]", err.Error())
	}

	// #nosec G104 -- error is not actionable here
	_ = dnssrv.Register()
	registerService := func(name string, port int) {}

	rejecter, err := revoker.Register(ctx, "inkgate-bf", cfg, l, registerService)
	if err != nil && !errors.Is(err, revoker.ErrNoConfig) {
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
