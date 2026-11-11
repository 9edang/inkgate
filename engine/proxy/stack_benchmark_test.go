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

var result any

func BenchmarkProxyStack_single(b *testing.B) {
	backend := &config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
		Host:            []string{"supu:8080"},
		Method:          "GET",
		URLPattern:      "/a/{{.Tupu}}",
		DenyList:        []string{"map.aaaa"},
		Mapping:         map[string]string{"supu": "SUPUUUUU"},
	}
	cfg := &config.EndpointConfig{
		Backend: []*config.Backend{backend},
		ExtraConfig: map[string]any{
			Namespace: map[string]any{
				staticKey: map[string]any{
					"data": map[string]any{
						"status": "errored",
					},
				},
				"strategy": "incomplete",
			},
		},
	}

	ef := NewEntityFormatter(backend)
	p := func(_ context.Context, _ *Request) (*Response, error) {
		res := ef.Format(Response{
			Data: map[string]any{
				"supu": 42,
				"tupu": true,
				"foo":  "bar",
				"map":  map[string]any{"aaaa": false},
				"col": []any{
					map[string]any{
						"a": 1,
						"b": 2,
					},
				},
			},
			IsComplete: true,
		})
		return &res, nil
	}
	p = NewRoundRobinLoadBalancedMiddleware(backend)(p)
	p = NewConcurrentMiddleware(backend)(p)
	p = NewRequestBuilderMiddleware(backend)(p)
	p = NewStaticMiddleware(logging.NoOp, cfg)(p)

	request := &Request{
		Method:  "GET",
		Body:    newDummyReadCloser(""),
		Params:  map[string]string{"Tupu": "true"},
		Headers: map[string][]string{},
	}

	var r *Response
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		r, _ = p(context.Background(), request)
	}
	result = r
}

func BenchmarkProxyStack_multi(b *testing.B) {
	backend := &config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
		Host:            []string{"supu:8080"},
		Method:          "GET",
		URLPattern:      "/a/{{.Tupu}}",
		DenyList:        []string{"map.aaaa"},
		Mapping:         map[string]string{"supu": "SUPUUUUU"},
	}

	request := &Request{
		Method:  "GET",
		Body:    newDummyReadCloser(""),
		Params:  map[string]string{"Tupu": "true"},
		Headers: map[string][]string{},
	}

	for _, testCase := range [][]*config.Backend{
		{backend},
		{backend, backend},
		{backend, backend, backend},
		{backend, backend, backend, backend},
		{backend, backend, backend, backend, backend},
	} {
		b.Run(fmt.Sprintf("with %d backends", len(testCase)), func(b *testing.B) {

			cfg := &config.EndpointConfig{
				Backend: testCase,
			}

			backendProxy := make([]Proxy, len(cfg.Backend))

			for i, backend := range cfg.Backend {
				ef := NewEntityFormatter(backend)
				backendProxy[i] = func(_ context.Context, _ *Request) (*Response, error) {
					res := ef.Format(Response{
						Data: map[string]any{
							"supu": 42,
							"tupu": true,
							"foo":  "bar",
							"map":  map[string]any{"aaaa": false},
							"col": []any{
								map[string]any{
									"a": 1,
									"b": 2,
								},
							},
						},
						IsComplete: true,
					})
					return &res, nil
				}
				backendProxy[i] = NewRoundRobinLoadBalancedMiddleware(backend)(backendProxy[i])
				backendProxy[i] = NewConcurrentMiddleware(backend)(backendProxy[i])
				backendProxy[i] = NewRequestBuilderMiddleware(backend)(backendProxy[i])
			}
			p := NewMergeDataMiddleware(logging.NoOp, cfg)(backendProxy...)
			p = NewStaticMiddleware(logging.NoOp, cfg)(p)

			var r *Response
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				r, _ = p(context.Background(), request)
			}
			result = r
		})
	}
}

func BenchmarkProxyStack_multipost(b *testing.B) {
	backendGET := &config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
		Host:            []string{"supu:8080"},
		Method:          "GET",
		URLPattern:      "/a/{{.Tupu}}",
		DenyList:        []string{"map.aaaa"},
		Mapping:         map[string]string{"supu": "SUPUUUUU"},
	}

	backendPOST := &config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
		Host:            []string{"supu:8080"},
		Method:          "POST",
		URLPattern:      "/a/{{.Tupu}}",
		DenyList:        []string{"map.aaaa"},
		Mapping:         map[string]string{"supu": "SUPUUUUU"},
	}

	request := &Request{
		Method:  "POST",
		Body:    newDummyReadCloser(""),
		Params:  map[string]string{"Tupu": "true"},
		Headers: map[string][]string{},
	}

	for _, testCase := range [][]*config.Backend{
		{backendGET},
		{backendGET, backendPOST},
		{backendGET, backendPOST, backendGET},
		{backendGET, backendPOST, backendGET, backendPOST},
		{backendGET, backendPOST, backendGET, backendPOST, backendPOST},
	} {
		b.Run(fmt.Sprintf("with %d backends", len(testCase)), func(b *testing.B) {

			cfg := &config.EndpointConfig{
				Backend: testCase,
			}

			backendProxy := make([]Proxy, len(cfg.Backend))

			for i, backend := range cfg.Backend {
				ef := NewEntityFormatter(backend)
				backendProxy[i] = func(_ context.Context, _ *Request) (*Response, error) {
					res := ef.Format(Response{
						Data: map[string]any{
							"supu": 42,
							"tupu": true,
							"foo":  "bar",
							"map":  map[string]any{"aaaa": false},
							"col": []any{
								map[string]any{
									"a": 1,
									"b": 2,
								},
							},
						},
						IsComplete: true,
					})
					return &res, nil
				}
				backendProxy[i] = NewRoundRobinLoadBalancedMiddleware(backend)(backendProxy[i])
				backendProxy[i] = NewConcurrentMiddleware(backend)(backendProxy[i])
				backendProxy[i] = NewRequestBuilderMiddleware(backend)(backendProxy[i])
			}
			p := NewMergeDataMiddleware(logging.NoOp, cfg)(backendProxy...)
			p = NewStaticMiddleware(logging.NoOp, cfg)(p)

			var r *Response
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				r, _ = p(context.Background(), request)
			}
			result = r
		})
	}
}

func BenchmarkProxyStack_single_flatmap(b *testing.B) {
	backend := &config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
		Host:            []string{"supu:8080"},
		Method:          "GET",
		URLPattern:      "/a/{{.Tupu}}",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]any{
				flatmapKey: []any{
					map[string]any{
						"type": "del",
						"args": []any{"map.aaaa"},
					},
					map[string]any{
						"type": "move",
						"args": []any{"supu", "SUPUUUUU"},
					},
				},
			},
		},
	}
	cfg := &config.EndpointConfig{
		Backend: []*config.Backend{backend},
		ExtraConfig: map[string]any{
			Namespace: map[string]any{
				staticKey: map[string]any{
					"data": map[string]any{
						"status": "errored",
					},
				},
				"strategy": "incomplete",
			},
		},
	}

	ef := NewEntityFormatter(backend)
	p := func(_ context.Context, _ *Request) (*Response, error) {
		res := ef.Format(Response{
			Data: map[string]any{
				"supu": 42,
				"tupu": true,
				"foo":  "bar",
				"map":  map[string]any{"aaaa": false},
				"col": []any{
					map[string]any{
						"a": 1,
						"b": 2,
					},
				},
			},
			IsComplete: true,
		})
		return &res, nil
	}
	p = NewRoundRobinLoadBalancedMiddleware(backend)(p)
	p = NewConcurrentMiddleware(backend)(p)
	p = NewRequestBuilderMiddleware(backend)(p)
	p = NewStaticMiddleware(logging.NoOp, cfg)(p)

	request := &Request{
		Method:  "GET",
		Body:    newDummyReadCloser(""),
		Params:  map[string]string{"Tupu": "true"},
		Headers: map[string][]string{},
	}

	var r *Response
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		r, _ = p(context.Background(), request)
	}
	result = r
}

func BenchmarkProxyStack_multi_flatmap(b *testing.B) {
	backend := &config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
		Host:            []string{"supu:8080"},
		Method:          "GET",
		URLPattern:      "/a/{{.Tupu}}",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]any{
				flatmapKey: []any{
					map[string]any{
						"type": "del",
						"args": []any{"map.aaaa"},
					},
					map[string]any{
						"type": "move",
						"args": []any{"supu", "SUPUUUUU"},
					},
				},
			},
		},
	}

	request := &Request{
		Method:  "GET",
		Body:    newDummyReadCloser(""),
		Params:  map[string]string{"Tupu": "true"},
		Headers: map[string][]string{},
	}

	for _, testCase := range [][]*config.Backend{
		{backend},
		{backend, backend},
		{backend, backend, backend},
		{backend, backend, backend, backend},
		{backend, backend, backend, backend, backend},
	} {
		b.Run(fmt.Sprintf("with %d backends", len(testCase)), func(b *testing.B) {

			cfg := &config.EndpointConfig{
				Backend: testCase,
			}

			backendProxy := make([]Proxy, len(cfg.Backend))

			for i, backend := range cfg.Backend {
				ef := NewEntityFormatter(backend)
				backendProxy[i] = func(_ context.Context, _ *Request) (*Response, error) {
					res := ef.Format(Response{
						Data: map[string]any{
							"supu": 42,
							"tupu": true,
							"foo":  "bar",
							"map":  map[string]any{"aaaa": false},
							"col": []any{
								map[string]any{
									"a": 1,
									"b": 2,
								},
							},
						},
						IsComplete: true,
					})
					return &res, nil
				}
				backendProxy[i] = NewRoundRobinLoadBalancedMiddleware(backend)(backendProxy[i])
				backendProxy[i] = NewConcurrentMiddleware(backend)(backendProxy[i])
				backendProxy[i] = NewRequestBuilderMiddleware(backend)(backendProxy[i])
			}
			p := NewMergeDataMiddleware(logging.NoOp, cfg)(backendProxy...)
			p = NewStaticMiddleware(logging.NoOp, cfg)(p)

			var r *Response
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				p(context.Background(), request)
			}
			result = r
		})
	}
}
