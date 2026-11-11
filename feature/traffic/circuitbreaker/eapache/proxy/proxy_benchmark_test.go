package proxy

import (
	"errors"
	"github.com/9edang/inkgate/context"
	"sync/atomic"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"

	"github.com/9edang/inkgate/feature/traffic/circuitbreaker/eapache"
)

func BenchmarkNewCircuitBreakerMiddleware_ok(b *testing.B) {
	p := NewMiddleware(&cfg)(dummyProxy(&proxy.Response{}, nil))
	for b.Loop() {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

func BenchmarkNewCircuitBreakerMiddleware_ko(b *testing.B) {
	p := NewMiddleware(&cfg)(dummyProxy(nil, errors.New("sample error")))
	for b.Loop() {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

func BenchmarkNewCircuitBreakerMiddleware_burst(b *testing.B) {
	err := errors.New("sample error")
	p := NewMiddleware(&cfg)(burstProxy(&proxy.Response{}, err, 100, 6))
	for b.Loop() {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

var cfg = config.Backend{
	ExtraConfig: map[string]any{
		eapache.Namespace: map[string]any{
			"error":   10.0,
			"success": 10.0,
			"timeout": "1s",
		},
	},
}

func burstProxy(r *proxy.Response, err error, ok, ko int) proxy.Proxy {
	tmp := make([]bool, ok+ko)
	for i := range ok + ko {
		tmp[i] = i < ok
	}
	calls := uint64(0)
	return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		total := atomic.AddUint64(&calls, 1) - 1
		if tmp[total%uint64(len(tmp))] {
			return r, nil
		}
		return nil, err
	}
}
