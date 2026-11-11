package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	gometrics "github.com/rcrowley/go-metrics"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/proxy"
)

// The cost the metrics add to one backend call: the proxy middleware around a proxy that answers at once.
func BenchmarkProxyMiddleware(b *testing.B) {
	r := gometrics.NewRegistry()
	pm := NewProxyMetrics(&r)
	resp := &proxy.Response{Data: map[string]any{}, IsComplete: true}
	p := NewProxyMiddleware("backend", "/users", pm)(func(context.Context, *proxy.Request) (*proxy.Response, error) {
		return resp, nil
	})
	ctx, req := context.Background(), &proxy.Request{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = p(ctx, req)
	}
}

// The cost the metrics add to one request at the router: status counter, size and time histograms.
func BenchmarkHTTPHandler(b *testing.B) {
	r := gometrics.NewRegistry()
	rm := NewRouterMetrics(&r)
	h := NewHTTPHandler("/users", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}), rm)
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	b.ReportAllocs()
	for b.Loop() {
		h(httptest.NewRecorder(), req)
	}
}

// Each outcome must increment the counter whose name says so; the handles are indexed, so a wrong index would
// silently mislabel every metric.
func TestProxyMiddlewareOutcomeNames(t *testing.T) {
	r := gometrics.NewRegistry()
	pm := NewProxyMetrics(&r)
	for _, tc := range []struct {
		resp *proxy.Response
		err  error
		name string
	}{
		{&proxy.Response{IsComplete: true}, nil, "complete.true.error.false"},
		{&proxy.Response{IsComplete: false}, nil, "complete.false.error.false"},
		{nil, errTest, "complete.false.error.true"},
		{&proxy.Response{IsComplete: true}, errTest, "complete.true.error.true"},
	} {
		p := NewProxyMiddleware("backend", "/x", pm)(func(context.Context, *proxy.Request) (*proxy.Response, error) {
			return tc.resp, tc.err
		})
		_, _ = p(context.Background(), &proxy.Request{})
		c, ok := r.Get("proxy.requests.layer.backend.name./x." + tc.name).(gometrics.Counter)
		if !ok || c.Count() != 1 {
			t.Errorf("%s: counter not incremented once (ok=%v)", tc.name, ok)
		}
	}
}

var errTest = errors.New("backend failed")

// Concurrent requests with different statuses: no count is lost and the status counters are created once.
func TestHTTPHandlerConcurrentStatuses(t *testing.T) {
	r := gometrics.NewRegistry()
	rm := NewRouterMetrics(&r)
	h := NewHTTPHandler("/c", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("fail") != "" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}), rm)

	var wg sync.WaitGroup
	for i := range 200 {
		wg.Go(func() {
			url := "/c"
			if i%4 == 0 {
				url += "?fail=1"
			}
			h(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, url, nil))
		})
	}
	wg.Wait()

	for status, want := range map[string]int64{"200": 150, "502": 50} {
		c, ok := r.Get("router.response./c.status." + status + ".count").(gometrics.Counter)
		if !ok || c.Count() != want {
			t.Errorf("status %s: ok=%v count=%v, want %d", status, ok, c, want)
		}
	}
}
