// SPDX-License-Identifier: Apache-2.0

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/transport/http/server"
)

// Through the real server, the way the binary runs it.
func TestReadyHandler_overTheWire(t *testing.T) {
	addr, shutdown := runGateway(t, http.HandlerFunc(ReadyHandler))
	resp, err := http.Get("http://" + addr + ReadyPath)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status %d, want 200", resp.StatusCode)
	}
	if _, err := shutdown(); err != nil {
		t.Fatal(err)
	}
}

// From the moment the shutdown starts the instance is not ready, whatever its backends do.
func TestReadyHandler_notReadyOnceShutdownStarts(t *testing.T) {
	drain := server.NewDrain()
	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, ReadyPath, http.NoBody)
		req = req.WithContext(server.WithDrain(req.Context(), drain))
		rec := httptest.NewRecorder()
		ReadyHandler(rec, req)

		return rec
	}
	if rec := get(); rec.Code != http.StatusOK {
		t.Fatalf("before the shutdown: %d, want 200", rec.Code)
	}
	drain.Start()
	rec := get()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("after the shutdown started: %d, want 503", rec.Code)
	}
	if rec.Header().Get("Connection") != "close" {
		t.Error("a not-ready answer must close the connection, so the next probe is balanced again")
	}
}

// A handler used outside a server (no drain in the context) is ready.
func TestReadyHandler_withoutServer(t *testing.T) {
	rec := httptest.NewRecorder()
	ReadyHandler(rec, httptest.NewRequest(http.MethodGet, ReadyPath, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Errorf("%d, want 200", rec.Code)
	}
}

// /__ready is reserved like the other built-in paths: an endpoint cannot take it.
func TestReady_pathIsReserved(t *testing.T) {
	cfg := config.ServiceConfig{Version: 3, Endpoints: []*config.EndpointConfig{{
		Endpoint: ReadyPath, Method: "GET", Backend: []*config.Backend{{URLPattern: "/x", Host: []string{"http://h"}}},
	}}}
	if err := cfg.Init(); err == nil {
		t.Error("an endpoint on /__ready must be refused")
	}
}
