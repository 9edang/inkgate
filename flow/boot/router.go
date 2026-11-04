package boot

import (
	"github.com/9edang/inkgate/context"
	"io"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	jose "github.com/9edang/inkgate/feature/auth/jose"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
)

// Router is what the HTTP side (flow/entry) plugs into boot.
// Everything router-independent (config, logging, telemetry, plugins, the
// endpoint and backend stages) is done by boot; flow/entry only builds the
// HTTP side: stage 2 (entry) and stage 3 (request).
type Router struct {
	// Name is the router name ("mux"); the binary is "inkgate-<Name>".
	Name string
	// NewMetrics creates the metrics collector and serves its stats endpoint
	// with this router.
	NewMetrics func(ctx context.Context, cfg config.ServiceConfig, l logging.Logger) *metrics.Metrics
	// RegisterEncoders adds router-specific output encodings (optional).
	RegisterEncoders func()
	// TestRoutes registers the endpoints on a real router, for `check --test-routes` (optional).
	TestRoutes func(cfg config.ServiceConfig) error
	// Serve builds stages 2 and 3 on top of s.EndpointFactory and blocks until ctx is done.
	Serve func(ctx context.Context, s Services)
}

// Services are the pieces boot hands to flow/entry.
type Services struct {
	Config config.ServiceConfig
	Logger logging.Logger
	// LogWriter is the GELF writer for access logs; nil when GELF is not configured.
	LogWriter io.Writer
	Metrics   *metrics.Metrics
	// TokenRejecter decides whether a valid JWT has been revoked.
	TokenRejecter jose.RejecterFactory
	// EndpointFactory builds stage 4 (and, inside it, stage 5) for an endpoint.
	EndpointFactory proxy.Factory
	// AgentPing receives a message each time an async agent reports healthy.
	AgentPing <-chan string
}
