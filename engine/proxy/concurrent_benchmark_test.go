// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"github.com/9edang/inkgate/context"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
)

func BenchmarkNewConcurrentMiddleware_singleNext(b *testing.B) {
	backend := config.Backend{
		ConcurrentCalls: 3,
		Timeout:         time.Duration(100) * time.Millisecond,
	}
	proxy := NewConcurrentMiddleware(&backend)(dummyProxy(&Response{}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		proxy(context.Background(), &Request{})
	}
}
