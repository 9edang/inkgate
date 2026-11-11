package ratelimit

import (
	"testing"
)

func BenchmarkTokenBucket(b *testing.B) {
	tb := NewTokenBucket(1000, 100)
	for b.Loop() {
		tb.Allow()
	}
}
