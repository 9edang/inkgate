package proxy

import (
	"errors"
	"github.com/9edang/inkgate/context"
	"sync/atomic"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"

	"github.com/9edang/inkgate/engine/logging"
	gcb "github.com/9edang/inkgate/feature/traffic/circuitbreaker/gobreaker"
)

func BenchmarkNewCircuitBreakerMiddleware_ok(b *testing.B) {
	p := NewMiddleware(&cfg, logging.NoOp)(dummyProxy(&proxy.Response{}, nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

func BenchmarkNewCircuitBreakerMiddleware_ko(b *testing.B) {
	p := NewMiddleware(&cfg, logging.NoOp)(dummyProxy(nil, errors.New("sample error")))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

func BenchmarkNewCircuitBreakerMiddleware_burst(b *testing.B) {
	err := errors.New("sample error")
	p := NewMiddleware(&cfg, logging.NoOp)(burstProxy(&proxy.Response{}, err, 100, 6))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

var cfg = config.Backend{
	ExtraConfig: map[string]any{
		gcb.Namespace: map[string]any{
			"interval":   100.0,
			"timeout":    100.0,
			"max_errors": 1.0,
		},
	},
}

func dummyProxy(r *proxy.Response, err error) proxy.Proxy {
	return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return r, err
	}
}

func burstProxy(r *proxy.Response, err error, ok, ko int) proxy.Proxy {
	tmp := make([]bool, ok+ko)
	for i := 0; i < ok+ko; i++ {
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
