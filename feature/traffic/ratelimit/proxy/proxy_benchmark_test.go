package proxy

import (
	"github.com/9edang/inkgate/context"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
)

func BenchmarkNewMiddleware_ok(b *testing.B) {
	p := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]any{
			Namespace: map[string]any{
				"max_rate": 10000000000000.0,
				"capacity": 100000000000.0,
			},
		},
	})(dummyProxy(&proxy.Response{}, nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}

func BenchmarkNewMiddleware_ko(b *testing.B) {
	p := NewMiddleware(logging.NoOp, &config.Backend{
		ExtraConfig: map[string]any{
			Namespace: map[string]any{
				"max_rate": 1.0,
				"capacity": 1.0,
			},
		},
	})(dummyProxy(&proxy.Response{}, nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p(context.Background(), &proxy.Request{
			Path: "/tupu",
		})
	}
}
