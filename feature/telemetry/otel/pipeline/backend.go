package pipeline

import (
	"github.com/9edang/inkgate/context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"

	coreconfig "github.com/9edang/inkgate/engine/config"
	transport "github.com/9edang/inkgate/engine/transport/http/client"

	otelconfig "github.com/9edang/inkgate/feature/telemetry/otel/config"
	clienthttp "github.com/9edang/inkgate/feature/telemetry/otel/http/client"
	otelstate "github.com/9edang/inkgate/feature/telemetry/otel/state"
)

var defaultOpts = otelconfig.BackendOpts{
	Metrics: &otelconfig.BackendMetricOpts{
		DisableStage:       false,
		RoundTrip:          true,
		ReadPayload:        true,
		DetailedConnection: true,
		StaticAttributes:   make(otelconfig.Attributes, 0),
	},
	Traces: &otelconfig.BackendTraceOpts{
		DisableStage:       false,
		RoundTrip:          true,
		ReadPayload:        true,
		DetailedConnection: true,
		StaticAttributes:   make(otelconfig.Attributes, 0),
	},
}

// HTTPRequestExecutorFromConfig creates an HTTPRequestExecutor to be used
// for the backend requests.
func HTTPRequestExecutorFromConfig(clientFactory transport.HTTPClientFactory,
	cfg *coreconfig.Backend,
) transport.HTTPRequestExecutor {
	cf := InstrumentedHTTPClientFactory(clientFactory, cfg)
	return transport.DefaultHTTPRequestExecutor(cf)
}

func InstrumentedHTTPClientFactory(clientFactory transport.HTTPClientFactory,
	cfg *coreconfig.Backend,
) transport.HTTPClientFactory {
	otelCfg := otelstate.GlobalConfig()
	if otelCfg == nil {
		return clientFactory
	}
	if otelCfg.SkipEndpoint(cfg.ParentEndpoint) {
		return clientFactory
	}

	strictSemConv := ""
	if g := otelCfg.GlobalOpts(); g != nil {
		strictSemConv = g.SemConv
	}

	opts := otelCfg.BackendOpts(cfg)
	if !opts.Enabled() {
		return clientFactory
	}
	otelState := otelCfg.BackendOTEL(cfg)

	// this might not be necessary:
	if opts.Metrics == nil {
		opts.Metrics = defaultOpts.Metrics
	}
	if opts.Traces == nil {
		opts.Traces = defaultOpts.Traces
	}

	urlPattern := otelconfig.NormalizeURLPattern(cfg.URLPattern)
	parentEndpoint := otelconfig.NormalizeURLPattern(cfg.ParentEndpoint)
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(cfg.Method),
		semconv.HTTPRoute(urlPattern), // <- for traces we can use URLFull to not have the matched path
		attribute.String("inkgate.endpoint.route", parentEndpoint),
		attribute.String("inkgate.endpoint.method", cfg.ParentEndpointMethod),
	}

	metricAttrs := attrs
	for _, kv := range opts.Metrics.StaticAttributes {
		if kv.Key != "" && kv.Value != "" {
			metricAttrs = append(metricAttrs, attribute.String(kv.Key, kv.Value))
		}
	}

	traceAttrs := make([]attribute.KeyValue, len(attrs),
		len(attrs)+1+len(opts.Traces.StaticAttributes))
	copy(traceAttrs, attrs)
	traceAttrs = append(traceAttrs,
		attribute.String("inkgate.stage", "backend-request"))
	for _, kv := range opts.Traces.StaticAttributes {
		if kv.Key != "" && kv.Value != "" {
			traceAttrs = append(traceAttrs, attribute.String(kv.Key, kv.Value))
		}
	}

	t := clienthttp.TransportOptions{
		MetricsOpts: clienthttp.TransportMetricsOptions{
			RoundTrip:          opts.Metrics.RoundTrip,
			ReadPayload:        opts.Metrics.ReadPayload,
			DetailedConnection: opts.Metrics.DetailedConnection,
			FixedAttributes:    metricAttrs,
			SemConv:            strictSemConv,
		},
		TracesOpts: clienthttp.TransportTracesOptions{
			RoundTrip:          opts.Traces.RoundTrip,
			ReadPayload:        opts.Traces.ReadPayload,
			DetailedConnection: opts.Traces.DetailedConnection,
			FixedAttributes:    traceAttrs,
			ReportHeaders:      opts.Traces.ReportHeaders,
			SkipHeaders:        opts.Traces.SkipHeaders,
		},
		OTELInstance: otelState,
	}

	return func(ctx context.Context) *http.Client {
		return clienthttp.InstrumentedHTTPClient(clientFactory(ctx), &t, urlPattern)
	}
}
