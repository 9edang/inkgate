// Client plugin of the benchmark, KrakenD style: the plain signature (no next stage) and the standard library only.
// It does exactly what the inkgate-style twin in ../client-inkgate does: it replaces the backend call with
// a call to an outbound API (the "url" setting) through its own pooled http.Client.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientRegisterer is the symbol both gateways look for.
var ClientRegisterer = registerer("bench-client")

type registerer string

func (registerer) RegisterLogger(any) {}

func (r registerer) RegisterClients(f func(
	name string,
	handler func(context.Context, map[string]any) (http.Handler, error),
)) {
	f(string(r), r.factory)
}

func (r registerer) factory(_ context.Context, extra map[string]any) (http.Handler, error) {
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
