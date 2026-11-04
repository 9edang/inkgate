package pipeline

import (
	"github.com/9edang/inkgate/context"
	"net/http"

	coreconfig "github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/router"
	kotelhttpserver "github.com/9edang/inkgate/feature/telemetry/otel/http/server"
	"github.com/9edang/inkgate/feature/telemetry/otel/state"
)

func GlobalRunServer(_ logging.Logger, next router.RunServerFunc) router.RunServerFunc {
	otelCfg := state.GlobalConfig()
	if otelCfg == nil {
		return next
	}

	return func(ctx context.Context, cfg coreconfig.ServiceConfig, h http.Handler) error {
		var trustedProxies []string
		if v, ok := cfg.ExtraConfig[router.Namespace].(map[string]any); ok {
			if tpxs, ok := v["trusted_proxies"].([]string); ok {
				trustedProxies = tpxs
			}
		}
		wrappedH := kotelhttpserver.NewTrackingHandlerWithTrustedProxies(h, trustedProxies)
		return next(ctx, cfg, wrappedH)
	}
}
