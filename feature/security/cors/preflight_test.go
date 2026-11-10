package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/engine/config"
)

// M1 1.2: rs/cors only accepts Access-Control-Request-Headers in the lower-case, sorted form browsers send. Other
// clients (curl, tests, hand-written clients) send other spellings; the preflight must pass for them too.
func TestPreflightWithExplicitAllowHeaders(t *testing.T) {
	m := NewMiddleware(config.ExtraConfig{Namespace: map[string]any{
		"allow_origins": []any{"http://example.com"},
		"allow_methods": []any{"GET", "POST"},
		"allow_headers": []any{"Origin", "Content-Type"},
	}}, nil)
	h := m.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for _, acrh := range []string{"content-type", "Content-Type", "Origin, Content-Type", "origin,content-type"} {
		r := httptest.NewRequest(http.MethodOptions, "/", nil)
		r.Header.Set("Origin", "http://example.com")
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", acrh)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
			t.Errorf("%q: allow-origin %q, want http://example.com", acrh, got)
		}
	}

	// a header outside the list is still refused
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	r.Header.Set("Origin", "http://example.com")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "X-Secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("X-Secret must be refused, got allow-origin %q", got)
	}
}
