package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	corerouter "github.com/9edang/inkgate/engine/router"
)

func TestNewEngine_fallbacks(t *testing.T) {
	cfg := config.ServiceConfig{ExtraConfig: config.ExtraConfig{corerouter.Namespace: map[string]any{
		"error_body": map[string]any{"404": map[string]any{"error": "nope"}},
	}}}
	e := NewEngine(cfg)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	e.Handle("/static", http.MethodGet, ok)
	e.Handle("/users/{id}", http.MethodGet, ok)
	e.Handle("/users/{id}", http.MethodPost, ok)

	for _, tc := range []struct {
		method, path string
		status       int
		location     string
		body         string
	}{
		{http.MethodGet, "/static", http.StatusOK, "", "ok"},
		{http.MethodGet, "/static/", http.StatusMovedPermanently, "/static", ""},
		{http.MethodGet, "/StAtic", http.StatusMovedPermanently, "/static", ""},
		{http.MethodPost, "/users/1/", http.StatusTemporaryRedirect, "/users/1", ""},
		{http.MethodPost, "/static", http.StatusMethodNotAllowed, "", "405 method not allowed"},
		{http.MethodGet, "/users/abc/", http.StatusMovedPermanently, "/users/abc", ""},
		{http.MethodGet, "/missing", http.StatusNotFound, "", `{"error":"nope"}`},
	} {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, w.Code, tc.status)
		}
		if tc.location != "" && w.Header().Get("Location") != tc.location {
			t.Errorf("%s %s: location %q, want %q", tc.method, tc.path, w.Header().Get("Location"), tc.location)
		}
		if tc.body != "" && w.Body.String() != tc.body {
			t.Errorf("%s %s: body %q, want %q", tc.method, tc.path, w.Body.String(), tc.body)
		}
	}
}
