// SPDX-License-Identifier: Apache-2.0

package router

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sync"

	gorilla "github.com/gorilla/mux"

	"github.com/9edang/inkgate/engine/transport/http/server"
)

// Engine defines the minimun required interface for the router engine.
type Engine interface {
	http.Handler
	Handle(pattern, method string, handler http.Handler)
}

// NewHTTPErrorInterceptor returns a HTTPErrorInterceptor over the injected response writer.
func NewHTTPErrorInterceptor(w http.ResponseWriter) *HTTPErrorInterceptor {
	return &HTTPErrorInterceptor{w, new(sync.Once)}
}

// HTTPErrorInterceptor is a reposnse writer that adds a header signaling incomplete response in case of
// seeing a status code not equal to 200.
type HTTPErrorInterceptor struct {
	http.ResponseWriter
	once *sync.Once
}

// WriteHeader records the status code and adds a header signaling incomplete responses.
func (i *HTTPErrorInterceptor) WriteHeader(code int) {
	i.once.Do(func() {
		if code != http.StatusOK {
			i.ResponseWriter.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
		}
	})
	i.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the wrapped writer, so http.ResponseController can reach it
// (flushing, write deadlines) through the interceptor.
func (i *HTTPErrorInterceptor) Unwrap() http.ResponseWriter { return i.ResponseWriter }

// Hijack lets the connection be taken over (WebSocket upgrades).
func (i *HTTPErrorInterceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) { //nolint:revive // the net/http.Hijacker signature
	if h, ok := i.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

// Flush sends any buffered data to the client.
func (i *HTTPErrorInterceptor) Flush() {
	if f, ok := i.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// GorillaEngine is the Engine of inkgate: a gorilla/mux router. Each
// endpoint is one route restricted to its method; path variables ({id},
// {id:[0-9]+}) are read with gorilla's Vars.
type GorillaEngine struct {
	router *gorilla.Router

	// set by NewEngine (options.go); DefaultEngine keeps plain gorilla behaviour
	interceptErrors       bool
	redirectTrailingSlash bool
	redirectFixedPath     bool
	notFound              http.HandlerFunc
}

// DefaultEngine returns a gorilla/mux engine without inkgate's fallbacks: a
// plain 404 for unknown paths and 405 for known paths with another method.
func DefaultEngine() *GorillaEngine {
	r := gorilla.NewRouter()
	r.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
		http.Error(w, "", http.StatusMethodNotAllowed)
	})

	return &GorillaEngine{router: r, interceptErrors: true}
}

// Handle registers a handler for a path pattern and an HTTP method. An invalid
// pattern (for example a bad {name:regexp}) panics, like a gin route conflict,
// so it fails at startup and in `check --test-routes` instead of being dropped.
func (e *GorillaEngine) Handle(pattern, method string, handler http.Handler) {
	if err := e.router.Handle(pattern, handler).Methods(method).GetError(); err != nil {
		panic(fmt.Sprintf("invalid route %s %s: %v", method, pattern, err))
	}
}

// ServeHTTP dispatches the request through the gorilla router.
func (e *GorillaEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !e.interceptErrors {
		e.router.ServeHTTP(w, r)
		return
	}
	e.router.ServeHTTP(NewHTTPErrorInterceptor(w), r)
}
