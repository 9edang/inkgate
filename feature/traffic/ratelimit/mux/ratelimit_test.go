package mux

import (
	"github.com/9edang/inkgate/context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	mux "github.com/9edang/inkgate/engine/router/mux"
	"github.com/9edang/inkgate/feature/traffic/ratelimit/router"
)

func newHandler(t *testing.T, rl map[string]any) http.HandlerFunc {
	t.Helper()
	cfg := &config.EndpointConfig{
		Endpoint: "/test",
		Method:   http.MethodGet,
	}
	if rl != nil {
		cfg.ExtraConfig = config.ExtraConfig{router.Namespace: rl}
	}
	ok := func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return &proxy.Response{Data: map[string]any{"ok": true}, IsComplete: true}, nil
	}
	return NewRateLimiterMw(logging.NoOp, mux.EndpointHandler)(cfg, ok)
}

func statuses(h http.HandlerFunc, n int, ip string) []int {
	out := make([]int, n)
	for i := range out {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		h(w, req)
		out[i] = w.Code
	}
	return out
}

func TestEndpointLimit(t *testing.T) {
	got := statuses(newHandler(t, map[string]any{"max_rate": 1.0, "capacity": 1.0}), 3, "1.1.1.1")
	if got[0] != http.StatusOK || got[1] != http.StatusServiceUnavailable || got[2] != http.StatusServiceUnavailable {
		t.Errorf("unexpected statuses %v", got)
	}
}

func TestClientLimitIsPerIP(t *testing.T) {
	h := newHandler(t, map[string]any{"client_max_rate": 1.0, "client_capacity": 1.0, "strategy": "ip"})
	if got := statuses(h, 2, "1.1.1.1"); got[0] != http.StatusOK || got[1] != http.StatusTooManyRequests {
		t.Errorf("first client: unexpected statuses %v", got)
	}
	if got := statuses(h, 1, "2.2.2.2"); got[0] != http.StatusOK {
		t.Errorf("second client should not be limited: %v", got)
	}
}

func TestNoConfigIsUntouched(t *testing.T) {
	h := newHandler(t, nil)
	if got := statuses(h, 5, "1.1.1.1"); got[4] != http.StatusOK {
		t.Errorf("unexpected statuses %v", got)
	}
}
