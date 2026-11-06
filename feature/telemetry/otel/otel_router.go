// Package mux is the gorilla/mux adapter of the OpenTelemetry endpoint
// instrumentation.
package kotel

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
	kotelconfig "github.com/9edang/inkgate/feature/telemetry/otel/config"
	kotelserver "github.com/9edang/inkgate/feature/telemetry/otel/http/server"
	otelstate "github.com/9edang/inkgate/feature/telemetry/otel/state"
)

// New wraps a handler factory so every request reports the matched endpoint
// pattern and the configured static attributes to the server-level span.
func New(hf router.HandlerFactory) router.HandlerFactory {
	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		next := hf(cfg, p)
		otelCfg := otelstate.GlobalConfig()
		if otelCfg == nil || otelCfg.SkipEndpoint(cfg.Endpoint) {
			return next
		}
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.Endpoint)

		var metricsAttrs, tracesAttrs []attribute.KeyValue
		if cfgExtra, err := kotelconfig.GatewayLayerExtraCfg(cfg.ExtraConfig); err == nil && cfgExtra.Global != nil {
			for _, kv := range cfgExtra.Global.MetricsStaticAttributes {
				if len(kv.Key) > 0 && len(kv.Value) > 0 {
					metricsAttrs = append(metricsAttrs, attribute.String(kv.Key, kv.Value))
				}
			}
			for _, kv := range cfgExtra.Global.TracesStaticAttributes {
				if len(kv.Key) > 0 && len(kv.Value) > 0 {
					tracesAttrs = append(tracesAttrs, attribute.String(kv.Key, kv.Value))
				}
			}
		}

		return func(w http.ResponseWriter, r *http.Request) {
			kotelserver.SetEndpointPattern(r.Context(), urlPattern)
			kotelserver.SetStaticAttributtes(r.Context(), metricsAttrs, tracesAttrs)
			next(w, r)
		}
	}
}
