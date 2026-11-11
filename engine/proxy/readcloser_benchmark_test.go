package proxy

import (
	"io"
	"strings"
	"testing"

	"github.com/9edang/inkgate/context"
)

// One no-op response body: wrapped so it is closed when the request ends. Kept as the evidence for M2 2.3: the
// goroutine-per-response wrapper (~0.37 µs) measured faster than context.AfterFunc variants (0.56-0.79 µs).
func BenchmarkReadCloserWrapper(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		ctx, cancel := context.WithCancel(context.Background())
		r := NewReadCloserWrapper(ctx, io.NopCloser(strings.NewReader("body")))
		_, _ = io.Copy(io.Discard, r)
		cancel()
	}
}
