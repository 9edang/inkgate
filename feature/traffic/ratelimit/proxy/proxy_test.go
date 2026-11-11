package proxy

import (
	"github.com/9edang/inkgate/context"
	"sync/atomic"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	limiter "github.com/9edang/inkgate/feature/traffic/ratelimit"
)

/*
In previous versions of inkgate, the returned EmptyMiddleware
used to panic when too many proxies were provided. However,
the behaviour has changed, and now logs with a Fatal, that
instead of panicking exits:
func TestNewMiddleware_multipleNext(t *testing.T) {
	defer func() {
		if r := recover(); r != proxy.ErrTooManyProxies {
			t.Errorf("The code did not panic\n")
		}
	}()
	NewMiddleware(logging.NoOp, &config.Backend{})(proxy.NoopProxy, proxy.NoopProxy)
}
*/

func TestNewMiddleware_zeroConfig(t *testing.T) {
	for _, cfg := range []*config.Backend{
		{},
		{ExtraConfig: map[string]any{Namespace: 42}},
	} {
		resp := proxy.Response{}
		mdw := NewMiddleware(logging.NoOp, cfg)
		p := mdw(dummyProxy(&resp, nil))

		request := proxy.Request{
			Path: "/tupu",
		}

		for range 100 {
			r, err := p(context.Background(), &request)
			if err != nil {
				t.Error(err.Error())
				return
			}
			if &resp != r {
				t.Fail()
			}
		}
	}
}

func TestNewMiddleware_ok(t *testing.T) {
	resp := proxy.Response{}
	mdw := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]any{Namespace: map[string]any{"max_rate": 10000.0, "capacity": 10000}},
	})
	p := mdw(dummyProxy(&resp, nil))

	request := proxy.Request{
		Path: "/tupu",
	}

	for range 1000 {
		r, err := p(context.Background(), &request)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if &resp != r {
			t.Fail()
		}
	}
}

func TestNewMiddleware_capacity(t *testing.T) {
	resp := proxy.Response{}
	mdw := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]any{Namespace: map[string]any{"max_rate": 100000.0, "every": "10s"}},
	})
	p := mdw(dummyProxy(&resp, nil))

	request := proxy.Request{
		Path: "/tupu",
	}

	for range 10000 {
		r, err := p(context.Background(), &request)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if &resp != r {
			t.Fail()
		}
	}
}

func TestNewMiddleware_ko(t *testing.T) {
	expected := proxy.Response{}
	calls := uint64(0)
	mdw := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]any{Namespace: map[string]any{"max_rate": 1.0, "capacity": 1.0}},
	})
	p := mdw(func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		total := atomic.AddUint64(&calls, 1)
		if total > 2 {
			t.Error("This proxy shouldn't been executed!")
		}
		return &expected, nil
	})

	request := proxy.Request{
		Path: "/tupu",
	}

	for range 100 {
		p(context.Background(), &request)
	}

	r, err := p(context.Background(), &request)
	if err != limiter.ErrLimited {
		t.Errorf("error expected")
	}
	if nil != r {
		t.Error("unexpected response")
	}
	if calls != 1 {
		t.Error("unexpected number of calls to the proxy")
	}
}

func dummyProxy(r *proxy.Response, err error) proxy.Proxy {
	return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return r, err
	}
}
