// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"github.com/9edang/inkgate/context"
	"testing"

	"github.com/9edang/inkgate/engine/config"
)

func BenchmarkNewRequestBuilderMiddleware(b *testing.B) {
	backend := config.Backend{
		URLPattern: "/supu",
		Method:     "GET",
	}
	proxy := NewRequestBuilderMiddleware(&backend)(dummyProxy(&Response{}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		proxy(context.Background(), &Request{})
	}
}
