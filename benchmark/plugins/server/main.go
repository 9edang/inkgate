// Server plugin of the benchmark: wraps the next handler and adds one response header.
// It uses only the standard library, so the same source is built for inkgate and for KrakenD.
package main

import (
	"context"
	"net/http"
)

// HandlerRegisterer is the symbol both gateways look for.
var HandlerRegisterer = registerer("bench-server")

type registerer string

func (registerer) RegisterLogger(any) {}

func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]any, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.handler)
}

func (registerer) handler(_ context.Context, _ map[string]any, next http.Handler) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Bench-Server", "1")
		next.ServeHTTP(w, req)
	}), nil
}

func main() {}
