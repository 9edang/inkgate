// Package ratelimit contains token-bucket rate limiters and their storage backends.
package ratelimit

import (
	"errors"
	"github.com/9edang/inkgate/context"
	"time"
)

var (
	// ErrLimited is the error returned when the rate limit has been exceded
	ErrLimited = errors.New("rate limit exceded")

	// DataTTL is the default eviction time
	DataTTL = 10 * time.Minute

	// DefaultShards are the number of shards to create by default
	DefaultShards uint64 = 2048

	now = time.Now
)

// Limiter defines a simple interface for a rate limiter
type Limiter interface {
	Allow() bool
}

// LimiterStore defines the interface for a limiter lookup function
type LimiterStore func(string) Limiter

// Backend is the interface of the persistence layer
type Backend interface {
	Load(string, func() any) any
	Store(string, any) error
}

// DefaultShardedMemoryBackend is a 2048 sharded ShardedMemoryBackend
func DefaultShardedMemoryBackend(ctx context.Context) *ShardedMemoryBackend {
	return NewShardedMemoryBackend(ctx, DefaultShards, DataTTL, PseudoFNV64a)
}
