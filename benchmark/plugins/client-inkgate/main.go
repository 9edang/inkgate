// Client plugin of the benchmark, inkgate style: the chainable signature that receives the next stage and the
// inkgate context package. It does exactly what the KrakenD-style twin in ../client-krakend does: it replaces the
// backend call with a call to an outbound API (the "url" setting) through its own pooled http.Client, and never
// calls next.
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/9edang/inkgate/context"
)

// ClientRegisterer is the symbol the inkgate plugin loader looks for.
var ClientRegisterer = registerer("bench-client")

type registerer string

func (registerer) RegisterLogger(any) {}

func (r registerer) RegisterClients(f func(
	name string,
	handler func(context.Context, map[string]any, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.factory)
}

func (r registerer) factory(_ context.Context, extra map[string]any, _ http.Handler) (http.Handler, error) {
	if name, _ := extra["name"].(string); name != string(r) {
		return nil, fmt.Errorf("unknown register %v", extra["name"])
	}
	settings, _ := extra[string(r)].(map[string]any)
	target, _ := settings["url"].(string)
	if target == "" {
		return nil, errors.New(`bench-client needs a "url" setting`)
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        256,
			MaxIdleConnsPerHost: 256,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		out, err := http.NewRequestWithContext(req.Context(), http.MethodGet, target, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		resp, err := client.Do(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}), nil
}

func main() {}
