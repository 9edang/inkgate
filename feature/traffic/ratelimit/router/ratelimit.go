// Package mux is the gorilla/mux adapter of the endpoint rate limit
// (qos/ratelimit/router).
package router

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	gorilla "github.com/gorilla/mux"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
	limiter "github.com/9edang/inkgate/feature/traffic/ratelimit"
)

// ErrNotFound is returned for an unknown client strategy.
var ErrNotFound = errors.New("not found")

// TokenExtractor returns the key that identifies a client.
type TokenExtractor func(*http.Request) string

// NewRateLimiterMw adds the endpoint and per-client rate limits configured in
// qos/ratelimit/router. Endpoints without the namespace are returned untouched.
func NewRateLimiterMw(logger logging.Logger, next router.HandlerFactory) router.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		logPrefix := "[ENDPOINT: " + remote.Endpoint + "][Ratelimit]"
		handler := next(remote, p)

		cfg, err := ConfigGetter(remote.ExtraConfig)
		if err != nil {
			if err != ErrNoExtraCfg {
				logger.Error(logPrefix, err)
			}
			return handler
		}
		return applyClientRateLimit(logger, logPrefix, cfg, applyGlobalRateLimit(logger, logPrefix, cfg, handler))
	}
}

func applyGlobalRateLimit(logger logging.Logger, logPrefix string, cfg Config, next http.HandlerFunc) http.HandlerFunc {
	if cfg.MaxRate <= 0 {
		return next
	}
	if cfg.Capacity == 0 {
		if cfg.MaxRate < 1 {
			cfg.Capacity = 1
		} else {
			cfg.Capacity = uint64(cfg.MaxRate)
		}
	}
	logger.Debug(logPrefix, fmt.Sprintf("Rate limit enabled. MaxRate: %f, Capacity: %d", cfg.MaxRate, cfg.Capacity))

	tb := limiter.NewTokenBucket(cfg.MaxRate, cfg.Capacity)
	return func(w http.ResponseWriter, r *http.Request) {
		if !tb.Allow() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		next(w, r)
	}
}

func applyClientRateLimit(logger logging.Logger, logPrefix string, cfg Config, next http.HandlerFunc) http.HandlerFunc {
	if cfg.ClientMaxRate <= 0 {
		return next
	}
	if cfg.ClientCapacity == 0 {
		if cfg.ClientMaxRate < 1 {
			cfg.ClientCapacity = 1
		} else {
			cfg.ClientCapacity = uint64(cfg.ClientMaxRate)
		}
	}
	tokenExtractor, err := TokenExtractorFromCfg(cfg)
	if err != nil {
		logger.Warning(logPrefix, "Unknown strategy", cfg.Strategy)
		return next
	}
	logger.Debug(logPrefix,
		fmt.Sprintf("Rate limit enabled. Strategy: %s (key: %s), MaxRate: %f, Capacity: %d",
			cfg.Strategy, cfg.Key, cfg.ClientMaxRate, cfg.ClientCapacity))

	store := StoreFromCfg(cfg)
	return func(w http.ResponseWriter, r *http.Request) {
		key := tokenExtractor(r)
		if key == "" || !store(key).Allow() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// TokenExtractorFromCfg selects how clients are identified: ip, header or param.
func TokenExtractorFromCfg(cfg Config) (TokenExtractor, error) {
	switch strings.ToLower(cfg.Strategy) {
	case "ip":
		return newIPTokenExtractor(cfg.Key), nil
	case "header":
		return func(r *http.Request) string { return r.Header.Get(cfg.Key) }, nil
	case "param":
		return func(r *http.Request) string { return gorilla.Vars(r)[cfg.Key] }, nil
	default:
		return nil, ErrNotFound
	}
}

// newIPTokenExtractor uses the first IP in header (when set and valid), else the client IP.
func newIPTokenExtractor(header string) TokenExtractor {
	return func(r *http.Request) string {
		if header != "" {
			if v := strings.TrimSpace(strings.Split(r.Header.Get(header), ",")[0]); v != "" {
				if ip := strings.Split(v, ":")[0]; net.ParseIP(ip) != nil {
					return ip
				}
			}
		}
		return router.ClientIP(r)
	}
}
