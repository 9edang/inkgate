// SPDX-License-Identifier: Apache-2.0

package boot

import (
	"sort"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/router"
	otelconfig "github.com/9edang/inkgate/feature/telemetry/otel/config"
	ratelimitproxy "github.com/9edang/inkgate/feature/traffic/ratelimit/proxy"
	ratelimitrouter "github.com/9edang/inkgate/feature/traffic/ratelimit/router"
)

// unaliased are the namespaces whose code name is also the name used in a config file, so they have no entry in aliases.
var unaliased = []string{
	ratelimitproxy.Namespace,
	ratelimitrouter.Namespace,
	otelconfig.Namespace,
	"backend/http/client", // engine/proxy clientHTTPOptions: send_body_on_redirect
}

// KnownNamespaces returns every extra_config namespace this build reads, as a config file may write it: the alias
// ("router/stream") or the code name ("inkgate/engine/router/stream"). A key outside this list turns its feature off
// without a message, so `inkgate check` and `inkgate run` refuse it.
//
// namespaces_test.go keeps the list complete: it fails when a `Namespace` constant appears in the code that is neither
// here nor in the list of internal names.
func KnownNamespaces() []string {
	set := map[string]bool{}
	for long, alias := range aliases {
		set[long], set[alias] = true, true
	}
	for _, n := range unaliased {
		set[n] = true
	}
	list := make([]string, 0, len(set))
	for n := range set {
		list = append(list, n)
	}
	sort.Strings(list)

	return list
}

// validateEndpoints checks what the router would only find out when it builds each endpoint, so `check` reports it
// too and `run` stops before it listens.
func validateEndpoints(cfg config.ServiceConfig) []error {
	var errs []error
	for _, e := range cfg.Endpoints {
		if e == nil {
			continue
		}
		if err := router.ValidateStream(e); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
