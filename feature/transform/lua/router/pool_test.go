package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
)

func luaHandler(tb testing.TB, extra map[string]any, next http.HandlerFunc) http.HandlerFunc {
	tb.Helper()
	cfg := &config.EndpointConfig{Endpoint: "/", ExtraConfig: config.ExtraConfig{Namespace: extra}}
	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc { return next }

	return HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)
}

// One request through an endpoint-level script that reads and sets a header.
func BenchmarkHandlerFactory(b *testing.B) {
	h := luaHandler(b, map[string]any{
		"pre": `local c = ctx.load()
c:headers("X-From-Lua", c:method())`,
	}, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h(httptest.NewRecorder(), req)
	}
}

// A global set by one request is gone in the next.
func TestRouterPoolGlobalsDoNotLeak(t *testing.T) {
	h := luaHandler(t, map[string]any{
		"pre": `if leaked ~= nil then custom_error("global leaked", 500) end
leaked = true`,
	}, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for i := range 5 {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d (%s)", i, w.Code, w.Body)
		}
	}
}

// Concurrent requests each see their own request.
func TestRouterPoolConcurrentRequestsAreIsolated(t *testing.T) {
	h := luaHandler(t, map[string]any{
		"pre": `local c = ctx.load()
c:headers("X-Echo", c:headers("X-Id"))`,
	}, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Echo") != r.Header.Get("X-Id") {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	var wg sync.WaitGroup
	codes := make(chan int, 200)
	for i := range 200 {
		wg.Go(func() {
			r := httptest.NewRequest(http.MethodGet, "/x", nil)
			r.Header.Set("X-Id", strconv.Itoa(i))
			w := httptest.NewRecorder()
			h(w, r)
			codes <- w.Code
		})
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Fatalf("a request saw another request's data (status %d)", code)
		}
	}
}
