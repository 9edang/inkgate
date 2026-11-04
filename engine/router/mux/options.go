package mux

import (
	"github.com/9edang/inkgate/internal/json"
	"net/http"
	"strings"
	"sync"
	"time"

	gorilla "github.com/gorilla/mux"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/core"
	corerouter "github.com/9edang/inkgate/engine/router"
	"github.com/9edang/inkgate/engine/transport/http/server"
)

// ReturnErrorMsg makes endpoint handlers write the error message in the body
// (router option return_error_msg). Set by NewEngine.
var ReturnErrorMsg bool

// Options are the router options (extra_config "router") the mux variant supports.
type Options struct {
	ReturnErrorMsg               bool   `json:"return_error_msg"`
	DisableHealth                bool   `json:"disable_health"`
	HealthPath                   string `json:"health_path"`
	DisableRedirectTrailingSlash bool   `json:"disable_redirect_trailing_slash"`
	DisableRedirectFixedPath     bool   `json:"disable_redirect_fixed_path"`
	ErrorBody                    struct {
		Err404 any `json:"404"`
		Err405 any `json:"405"`
	} `json:"error_body"`
}

// ParseOptions reads the router options from the service config.
func ParseOptions(cfg config.ServiceConfig) Options {
	var opts Options
	if v, ok := cfg.ExtraConfig[corerouter.Namespace]; ok && v != nil {
		if b, err := json.Marshal(v); err == nil {
			json.Unmarshal(b, &opts)
		}
	}
	return opts
}

// NewEngine returns a GorillaEngine configured from the router options: custom
// 404/405 bodies, trailing-slash redirects and return_error_msg. It behaves
// consistently with other inkgate builds.
func NewEngine(cfg config.ServiceConfig) *GorillaEngine {
	opts := ParseOptions(cfg)
	ReturnErrorMsg = opts.ReturnErrorMsg

	e := DefaultEngine()
	e.interceptErrors = false
	e.redirectTrailingSlash = !opts.DisableRedirectTrailingSlash
	e.redirectFixedPath = !opts.DisableRedirectFixedPath
	e.notFound = fallbackHandler(http.StatusNotFound, "404 page not found", opts.ErrorBody.Err404)
	e.router.NotFoundHandler = http.HandlerFunc(e.serveNoRoute)
	e.router.MethodNotAllowedHandler = fallbackHandler(http.StatusMethodNotAllowed, "405 method not allowed", opts.ErrorBody.Err405)
	return e
}

// HealthPath returns where the health endpoint is served, or "" when disabled.
func HealthPath(cfg config.ServiceConfig) string {
	opts := ParseOptions(cfg)
	if opts.DisableHealth {
		return ""
	}
	if opts.HealthPath != "" {
		return opts.HealthPath
	}
	return "/__health"
}

// NewHealthHandler answers {"status":"ok","agents":{...},"now":"..."}; agents
// are the async agents that reported on the channel, with the last report time.
func NewHealthHandler(health <-chan string) http.HandlerFunc {
	mu := new(sync.RWMutex)
	reports := map[string]string{}

	if health != nil {
		go func() {
			for name := range health {
				mu.Lock()
				reports[name] = time.Now().String()
				mu.Unlock()
			}
		}()
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		mu.RLock()
		body, _ := json.Marshal(map[string]any{"status": "ok", "agents": reports, "now": time.Now().String()})
		mu.RUnlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(body)
	}
}

func fallbackHandler(status int, text string, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(core.InkgateHeaderName, core.InkgateHeaderValue)
		w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
		if body != nil {
			b, _ := json.Marshal(body)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)
			w.Write(b)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		w.Write([]byte(text))
	}
}

// serveNoRoute handles paths with no endpoint. It first
// tries to redirect: to the same path with or without the trailing slash, then
// to the lower-case path (fixed path). Otherwise it answers 404.
func (e *GorillaEngine) serveNoRoute(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var candidates []string
	if e.redirectTrailingSlash && path != "/" {
		candidates = append(candidates, toggleSlash(path))
	}
	if e.redirectFixedPath {
		if lower := strings.ToLower(path); lower != path {
			candidates = append(candidates, lower)
			if e.redirectTrailingSlash && lower != "/" {
				candidates = append(candidates, toggleSlash(lower))
			}
		}
	}

	for _, alt := range candidates {
		probe := r.Clone(r.Context())
		probe.URL.Path = alt
		var match gorilla.RouteMatch
		if !e.router.Match(probe, &match) || match.MatchErr != nil {
			continue
		}
		code := http.StatusMovedPermanently
		if r.Method != http.MethodGet {
			code = http.StatusTemporaryRedirect
		}
		u := *r.URL
		u.Path = alt
		http.Redirect(w, r, u.String(), code)
		return
	}
	e.notFound(w, r)
}

func toggleSlash(p string) string {
	if strings.HasSuffix(p, "/") {
		return strings.TrimSuffix(p, "/")
	}
	return p + "/"
}
