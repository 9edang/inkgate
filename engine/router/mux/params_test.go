package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gorilla "github.com/gorilla/mux"
)

func extractFrom(t *testing.T, pattern, path string, wrap func(*http.Request) *http.Request) map[string]string {
	t.Helper()
	var got map[string]string
	r := gorilla.NewRouter()
	r.HandleFunc(pattern, func(_ http.ResponseWriter, req *http.Request) {
		got = PathParamExtractor(wrap(req))
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	return got
}

func TestPathParamExtractor(t *testing.T) {
	got := extractFrom(t, "/users/{id}/orders/{order_id:[0-9]+}", "/users/42/orders/7",
		func(r *http.Request) *http.Request { return r })
	if got["Id"] != "42" || got["Order_id"] != "7" || len(got) != 2 {
		t.Errorf("unexpected params: %v", got)
	}
}

func TestWithParams(t *testing.T) {
	got := extractFrom(t, "/users/{id}", "/users/42", func(r *http.Request) *http.Request {
		return WithParams(r, map[string]string{"JWT.sub": "alice"})
	})
	if got["Id"] != "42" || got["JWT.sub"] != "alice" {
		t.Errorf("unexpected params: %v", got)
	}
}
