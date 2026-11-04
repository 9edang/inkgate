package cors

import (
	"bufio"
	"github.com/9edang/inkgate/context"
	"io"
	"log"
	"net/http"

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

	if l == nil || !cfg.Debug {
		return c
	}

	r, w := io.Pipe()
	c.Log = log.New(w, "", log.LstdFlags)
	go writeLog(r, l)
	return c
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

func writeLog(r *io.PipeReader, l logging.Logger) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		l.Debug("[CORS]", scanner.Text())
	}
}
