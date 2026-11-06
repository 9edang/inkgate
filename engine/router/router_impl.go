// SPDX-License-Identifier: Apache-2.0

/*
Package mux is the inkgate router based on gorilla/mux
*/
package router

import (
	"net/http"
	"strings"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"

	"github.com/9edang/inkgate/engine/transport/http/server"
)

// DefaultDebugPattern is the default pattern used to define the debug endpoint.
const (
	DefaultDebugPattern = "/__debug/{params:.*}"
	DefaultEchoPattern  = "/__echo/{params:.*}"
	logPrefix           = "[SERVICE: Router]"
)

// Config is the struct that collects the parts the router should be builded from.
type Config struct {
	Engine         Engine
	Middlewares    []HandlerMiddleware
	HandlerFactory HandlerFactory
	ProxyFactory   proxy.Factory
	Logger         logging.Logger
	DebugPattern   string
	EchoPattern    string
	RunServer      RunServerFunc
	// HealthPath and HealthHandler replace the default /__health endpoint;
	// an empty HealthPath with a non-nil HealthHandler disables it.
	HealthPath    string
	HealthHandler http.Handler
}

// HandlerMiddleware is the interface for the decorators over the http.Handler.
type HandlerMiddleware interface {
	Handler(h http.Handler) http.Handler
}

// DefaultFactory returns a gorilla/mux router factory with the injected proxy factory and logger.
func DefaultFactory(pf proxy.Factory, logger logging.Logger) Factory {
	return factory{
		Config{
			Engine:         DefaultEngine(),
			Middlewares:    []HandlerMiddleware{},
			HandlerFactory: EndpointHandler,
			ProxyFactory:   pf,
			Logger:         logger,
			DebugPattern:   DefaultDebugPattern,
			EchoPattern:    DefaultEchoPattern,
			RunServer:      server.RunServer,
		},
	}
}

// NewFactory returns a gorilla/mux router factory with the injected configuration.
func NewFactory(cfg Config) Factory {
	if cfg.DebugPattern == "" {
		cfg.DebugPattern = DefaultDebugPattern
	}

	return factory{cfg}
}

type factory struct {
	cfg Config
}

// New implements the factory interface.
func (rf factory) New() Router {
	return rf.NewWithContext(context.Background())
}

// NewWithContext implements the factory interface.
func (rf factory) NewWithContext(ctx context.Context) Router {
	return httpRouter{rf.cfg, ctx, rf.cfg.RunServer}
}

type httpRouter struct {
	cfg       Config
	ctx       context.Context
	RunServer RunServerFunc
}

// HealthHandler is a dummy http.HandlerFunc implementation for exposing a health check endpoint.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(constant.HeaderContentType, constant.MIMEJSON)
	// #nosec G104 -- response write is best effort, the client may be gone
	w.Write([]byte(`{"status":"ok"}`))
}

// Run implements the router interface.
func (r httpRouter) Run(cfg config.ServiceConfig) {
	if cfg.Debug {
		debugHandler := DebugHandler(r.cfg.Logger)
		for _, method := range []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
			http.MethodConnect,
			http.MethodTrace,
		} {
			r.cfg.Engine.Handle(r.cfg.DebugPattern, method, debugHandler)
		}
	}

	if cfg.Echo {
		echoHandler := EchoHandler()
		for _, method := range []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
			http.MethodConnect,
			http.MethodTrace,
		} {
			r.cfg.Engine.Handle(r.cfg.EchoPattern, method, echoHandler)
		}
	}

	switch {
	case r.cfg.HealthHandler == nil:
		r.cfg.Engine.Handle("/__health", "GET", http.HandlerFunc(HealthHandler))
	case r.cfg.HealthPath != "":
		r.cfg.Engine.Handle(r.cfg.HealthPath, "GET", r.cfg.HealthHandler)
	}

	server.InitHTTPDefaultTransport(cfg)

	r.registerInkgateEndpoints(cfg.Endpoints)

	if err := r.RunServer(r.ctx, cfg, r.handler()); err != nil {
		r.cfg.Logger.Error(logPrefix, err.Error())
	}

	r.cfg.Logger.Info(logPrefix, "Router execution ended")
}

func (r httpRouter) registerInkgateEndpoints(endpoints []*config.EndpointConfig) {
	for _, c := range endpoints {
		proxyStack, err := r.cfg.ProxyFactory.New(c)
		if err != nil {
			r.cfg.Logger.Error(logPrefix, "Calling the ProxyFactory", err.Error())
			continue
		}

		r.registerInkgateEndpoint(c.Method, c, r.cfg.HandlerFactory(c, proxyStack), len(c.Backend))
	}
}

func (r httpRouter) registerInkgateEndpoint(method string, endpoint *config.EndpointConfig, handler http.HandlerFunc, totBackends int) {
	method = strings.ToTitle(method)
	path := endpoint.Endpoint
	if method != http.MethodGet && totBackends > 1 {
		if !IsValidSequentialEndpoint(endpoint) {
			r.cfg.Logger.Error(logPrefix, method, " endpoints with sequential proxy enabled only allow a non-GET in the last backend! Ignoring", path)
			return
		}
	}

	switch method {
	case http.MethodGet:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	default:
		r.cfg.Logger.Error(logPrefix, "Unsupported method", method)
		return
	}
	r.cfg.Logger.Debug(logPrefix, "Registering the endpoint", method, path)
	r.cfg.Engine.Handle(path, method, handler)
}

func (r httpRouter) handler() http.Handler {
	var handler http.Handler = r.cfg.Engine
	for _, middleware := range r.cfg.Middlewares {
		r.cfg.Logger.Debug(logPrefix, "Adding the middleware", middleware)
		handler = middleware.Handler(handler)
	}

	return handler
}
