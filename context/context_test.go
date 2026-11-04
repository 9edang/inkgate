package context

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFrom_survivesDerivedContexts(t *testing.T) {
	flow := New(Background(), Source{Kind: KindHTTP})
	ctx, cancel := WithTimeout(flow, time.Second)
	defer cancel()
	ctx = WithValue(ctx, "k", "v")
	ctx, cancel2 := WithCancel(ctx)
	defer cancel2()

	if got := From(ctx); got != flow {
		t.Fatalf("From returned %p, want %p", got, flow)
	}
	if From(Background()) != nil {
		t.Error("a plain context must not carry an flow context")
	}
}

func TestNilIsSafe(t *testing.T) {
	var flow *Flow
	flow.Set("a", 1)
	flow.SetClaims(map[string]any{"sub": "x"})
	flow.AddBackendCall(BackendCall{})
	if flow.RequestID() != "" || flow.Claims() != nil || len(flow.BackendCalls()) != 0 {
		t.Error("nil Flow must read zero values")
	}
	if _, ok := flow.Get("a"); ok {
		t.Error("nil Flow must not store values")
	}
}

func TestDataIsSharedAndCopied(t *testing.T) {
	flow := New(Background(), Source{})
	child, cancel := WithCancel(flow)
	defer cancel()

	From(child).Set("tenant", "acme")
	if v, _ := flow.Get("tenant"); v != "acme" {
		t.Errorf("value set through a derived context is not visible: %v", v)
	}

	params := map[string]string{"Id": "1"}
	flow.SetParams(params)
	params["Id"] = "changed"
	if flow.Params()["Id"] != "1" {
		t.Error("SetParams must copy the map")
	}
	flow.Params()["Id"] = "changed"
	if flow.Params()["Id"] != "1" {
		t.Error("Params must return a copy")
	}
}

func TestConcurrentUse(t *testing.T) {
	flow := New(Background(), Source{})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			flow.AddBackendCall(BackendCall{Backend: "/x"})
			flow.Set("k", i)
			flow.Get("k")
			flow.BackendCalls()
		}()
	}
	wg.Wait()
	if n := len(flow.BackendCalls()); n != 50 {
		t.Errorf("recorded %d backend calls, want 50", n)
	}
}

func TestMiddleware_requestID(t *testing.T) {
	var seen *Flow
	h := Middleware(ParseOptions(nil))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = From(r.Context())
	}))

	for _, tc := range []struct {
		name, sent string
		reuse      bool
	}{
		{"generated when absent", "", false},
		{"reused when valid", "abc-123", true},
		{"replaced when invalid", "has space", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
		if tc.sent != "" {
			req.Header.Set(DefaultRequestIDHeader, tc.sent)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		id := seen.RequestID()
		if id == "" || w.Header().Get(DefaultRequestIDHeader) != id {
			t.Errorf("%s: id %q, response header %q", tc.name, id, w.Header().Get(DefaultRequestIDHeader))
		}
		if tc.reuse != (id == tc.sent) {
			t.Errorf("%s: id %q, sent %q", tc.name, id, tc.sent)
		}
		if c := seen.Client(); c.Method != http.MethodGet || c.Path != "/users/1" {
			t.Errorf("%s: client %+v", tc.name, c)
		}
	}
}

func TestParseOptions(t *testing.T) {
	opts := ParseOptions(map[string]any{Namespace: map[string]any{
		"request_id_header": "X-Correlation-Id", "trust_request_id": false, "propagate_request_id": true,
	}})
	if opts.RequestIDHeader != "X-Correlation-Id" || opts.trust() || !opts.PropagateRequestID {
		t.Errorf("unexpected options %+v", opts)
	}
	if def := ParseOptions(nil); def.RequestIDHeader != DefaultRequestIDHeader || !def.trust() || def.PropagateRequestID {
		t.Errorf("unexpected defaults %+v", def)
	}
}

func TestPluginKey(t *testing.T) {
	flow := New(Background(), Source{RequestID: "req-9"})
	ctx, cancel := WithTimeout(flow, time.Second)
	defer cancel()

	// what a .so plugin does: no shared types, only a local interface
	type pluginFlow interface {
		RequestID() string
		Set(key string, value any)
	}
	f, ok := ctx.Value("inkgate.flow").(pluginFlow)
	if !ok {
		t.Fatal("the flow must be reachable with the plain string key")
	}
	f.Set("plugin.seen", true)
	if f.RequestID() != "req-9" {
		t.Errorf("request ID %q", f.RequestID())
	}
	if v, _ := flow.Get("plugin.seen"); v != true {
		t.Error("a value set by the plugin must be visible to the gateway")
	}
}
