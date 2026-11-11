// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

func BenchmarkNewMergeDataMiddleware(b *testing.B) {
	backend := config.Backend{}
	backends := make([]*config.Backend, 10)
	for i := range backends {
		backends[i] = &backend
	}

	proxies := []Proxy{
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
	}

	for _, totalParts := range []int{2, 3, 4, 5, 6, 7, 8, 9, 10} {
		b.Run(fmt.Sprintf("with %d parts", totalParts), func(b *testing.B) {
			endpoint := config.EndpointConfig{
				Backend: backends[:totalParts],
				Timeout: time.Duration(100) * time.Millisecond,
			}
			proxy := NewMergeDataMiddleware(logging.NoOp, &endpoint)(proxies[:totalParts]...)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				proxy(context.Background(), &Request{Params: map[string]string{}})
			}
		})
	}
}

func BenchmarkNewMergeDataMiddleware_sequential(b *testing.B) {
	backends := make([]*config.Backend, 10)
	pattern := "/some"
	var keys []string

	for i := range backends {
		b := &config.Backend{
			URLKeys:    make([]string, 4*i),
			URLPattern: pattern,
		}
		copy(b.URLKeys, keys)
		backends[i] = b
		for _, t := range []string{"int", "float", "bool", "string"} {
			next := fmt.Sprintf("Resp%d_%s", i, t)
			pattern += "/{{." + next + "}}"
			keys = append(keys, next)
		}
	}

	proxies := []Proxy{
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]any{"int": 1, "float": 3.14, "bool": true, "string": "wwwww"}, IsComplete: true}),
	}

	for _, totalParts := range []int{2, 3, 4, 5, 6, 7, 8, 9, 10} {
		b.Run(fmt.Sprintf("with %d parts", totalParts), func(b *testing.B) {
			endpoint := config.EndpointConfig{
				Backend: backends[:totalParts],
				Timeout: time.Duration(100) * time.Millisecond,
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]any{
						isSequentialKey: true,
					},
				},
			}
			proxy := NewMergeDataMiddleware(logging.NoOp, &endpoint)(proxies[:totalParts]...)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				proxy(context.Background(), &Request{Params: map[string]string{}})
			}
		})
	}
}
