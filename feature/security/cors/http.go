package cors

import (
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/9edang/inkgate/context"

	rscors "github.com/rs/cors"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/router"
)

// Middleware wraps an http.Handler. *rscors.Cors implements it.
type Middleware interface {
	Handler(h http.Handler) http.Handler
}

// NewMiddleware returns a net/http CORS middleware for the service config, or nil
// when CORS is not configured. Used by both router variants.
func NewMiddleware(e config.ExtraConfig, l logging.Logger) Middleware {
	tmp := ConfigGetter(e)
	if tmp == nil {
		return nil
	}
	cfg, ok := tmp.(Config)
	if !ok {
		return nil
	}
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = []string{"*"}
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = []string{"*"}
	}
	c := rscors.New(rscors.Options{
		AllowedOrigins:       cfg.AllowOrigins,
		AllowedMethods:       cfg.AllowMethods,
		AllowedHeaders:       cfg.AllowHeaders,
		ExposedHeaders:       cfg.ExposeHeaders,
		AllowCredentials:     cfg.AllowCredentials,
		AllowPrivateNetwork:  cfg.AllowPrivateNetwork,
		OptionsPassthrough:   cfg.OptionsPassthrough,
		OptionsSuccessStatus: cfg.OptionsSuccessStatus,
		Debug:                cfg.Debug,
		MaxAge:               int(cfg.MaxAge.Seconds()),
	})

	if l != nil && cfg.Debug {
		// written synchronously: rs/cors logs a decision before it answers, so the line is in the log by then
		c.Log = log.New(logWriter{l}, "", 0)
	}

	return preflightNormalizer{c}
}

// preflightNormalizer rewrites Access-Control-Request-Headers into the form browsers send (lower case, trimmed,
// sorted, comma separated) before rs/cors reads it. rs/cors (v1.11) only accepts that form, so a preflight from curl,
// a test or a hand-written client listing "Origin, Content-Type" would otherwise be refused although both are allowed.
type preflightNormalizer struct {
	cors *rscors.Cors
}

func (p preflightNormalizer) Handler(h http.Handler) http.Handler {
	next := p.cors.Handler(h)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			if v := r.Header.Values("Access-Control-Request-Headers"); len(v) > 0 {
				r.Header.Set("Access-Control-Request-Headers", normalizeHeaderList(v))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func normalizeHeaderList(values []string) string {
	var names []string
	for _, v := range values {
		for name := range strings.SplitSeq(v, ",") {
			if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)

	return strings.Join(slices.Compact(names), ",")
}

// NewRunServerWithLogger wraps the server start so CORS runs before the router
// selects a handler: preflight requests are answered for any path.
func NewRunServerWithLogger(next router.RunServerFunc, l logging.Logger) router.RunServerFunc {
	if l == nil {
		l = logging.NoOp
	}
	return func(ctx context.Context, cfg config.ServiceConfig, handler http.Handler) error {
		corsMw := NewMiddleware(cfg.ExtraConfig, l)
		if corsMw == nil {
			return next(ctx, cfg, handler)
		}
		l.Debug("[SERVICE: HTTP][CORS] Enabled CORS for all requests")
		return next(ctx, cfg, corsMw.Handler(handler))
	}
}

// logWriter writes the debug lines of rs/cors to the gateway logger, one Debug call per line.
type logWriter struct {
	l logging.Logger
}

func (w logWriter) Write(p []byte) (int, error) {
	for line := range strings.SplitSeq(strings.TrimRight(string(p), "\n"), "\n") {
		w.l.Debug("[CORS]", strings.TrimSpace(line))
	}

	return len(p), nil
}
