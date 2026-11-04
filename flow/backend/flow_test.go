package backend

import (
	"errors"
	"testing"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
)

func TestRecordFlow(t *testing.T) {
	var sentHeaders map[string][]string
	next := func(remote *config.Backend) proxy.Proxy {
		return func(_ context.Context, req *proxy.Request) (*proxy.Response, error) {
			sentHeaders = req.Headers
			if remote.URLPattern == "/fail" {
				return nil, errors.New("boom")
			}
			return &proxy.Response{IsComplete: true, Metadata: proxy.Metadata{StatusCode: 200}}, nil
		}
	}
	f := recordFlow(next)

	flow := context.New(context.Background(), context.Source{
		RequestID: "req-1",
		Options:   context.Options{PropagateRequestID: true},
	})
	original := map[string][]string{"Accept": {"*/*"}}
	if _, err := f(&config.Backend{URLPattern: "/ok"})(flow, &proxy.Request{Headers: original}); err != nil {
		t.Fatal(err)
	}
	f(&config.Backend{URLPattern: "/fail"})(flow, &proxy.Request{})

	if got := sentHeaders[context.DefaultRequestIDHeader]; len(got) != 1 || got[0] != "req-1" {
		t.Errorf("request ID not propagated: %v", got)
	}
	if _, ok := original[context.DefaultRequestIDHeader]; ok {
		t.Error("the caller's header map must not be modified")
	}
	calls := flow.BackendCalls()
	if len(calls) != 2 || calls[0].Backend != "/ok" || calls[0].Status != 200 || !calls[0].Complete ||
		calls[1].Backend != "/fail" || calls[1].Err != "boom" {
		t.Errorf("unexpected calls: %+v", calls)
	}
}

func TestRecordFlow_withoutContext(t *testing.T) {
	called := false
	next := func(*config.Backend) proxy.Proxy {
		return func(context.Context, *proxy.Request) (*proxy.Response, error) { called = true; return nil, nil }
	}
	recordFlow(next)(&config.Backend{})(context.Background(), &proxy.Request{})
	if !called {
		t.Error("next must be called when there is no flow context")
	}
}
