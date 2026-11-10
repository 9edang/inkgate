package boot

import (
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
)

// markJSONPassthrough turns the JSON pass-through on for the endpoints that allow it, when the service opts in with
// "router": {"json_passthrough": true}. Each decision is logged at DEBUG with what keeps an endpoint decoding.
func markJSONPassthrough(cfg config.ServiceConfig, logger logging.Logger) {
	opts, _ := cfg.ExtraConfig[router.Namespace].(map[string]any)
	if on, _ := opts["json_passthrough"].(bool); !on {
		return
	}
	for _, e := range cfg.Endpoints {
		ok, reason := proxy.JSONPassthrough(e)
		if ok {
			e.Backend[0].JSONPassthrough = true
			logger.Debug("[ENDPOINT: " + e.Endpoint + "] JSON pass-through: on")

			continue
		}
		logger.Debug("[ENDPOINT: " + e.Endpoint + "] JSON pass-through: off, " + reason)
	}
}
