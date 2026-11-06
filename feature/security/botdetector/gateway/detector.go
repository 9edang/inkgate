package gateway

import (
	"errors"
	"net/http"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
	botdetector "github.com/9edang/inkgate/feature/security/botdetector"
)

// New checks the configuration and, if required, wraps the handler factory with a bot detector middleware.
func New(hf router.HandlerFactory, l logging.Logger) router.HandlerFactory {
	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		next := hf(cfg, p)

		detectorCfg, err := ParseConfig(cfg.ExtraConfig)
		if errors.Is(err, ErrNoConfig) {
			l.Debug("botdetector: ", err.Error())
			return next
		}

		if err != nil {
			l.Warning("botdetector: ", err.Error())
			return next
		}

		d, err := botdetector.New(detectorCfg)
		if err != nil {
			l.Warning("botdetector: unable to create the LRU detector:", err.Error())
			return next
		}

		return handler(d, next, l)
	}
}

func handler(f botdetector.DetectorFunc, next http.HandlerFunc, l logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if f(r) {
			l.Error(errBotRejected)
			w.WriteHeader(http.StatusForbidden)

			return
		}
		next(w, r)
	}
}

type middleware struct {
	detector botdetector.DetectorFunc
	logger   logging.Logger
}

// NewMiddleware checks the configuration and, if required, registers a bot detector middleware at the router engine.
func NewMiddleware(cfg config.ExtraConfig, l logging.Logger) router.HandlerMiddleware {
	detectorCfg, err := ParseConfig(cfg)
	if errors.Is(err, ErrNoConfig) {
		l.Debug("botdetector middleware: ", err.Error())
		return nil
	}
	if err != nil {
		l.Warning("botdetector middleware: ", err.Error())
		return nil
	}
	d, err := botdetector.New(detectorCfg)
	if err != nil {
		l.Warning("botdetector middleware: unable to createt the LRU detector:", err.Error())
		return nil
	}

	return middleware{
		logger:   l,
		detector: d,
	}
}

func (m middleware) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.detector(r) {
			m.logger.Error(errBotRejected)
			w.WriteHeader(http.StatusForbidden)

			return
		}
		h.ServeHTTP(w, r)
	})
}

var errBotRejected = errors.New("bot rejected")
