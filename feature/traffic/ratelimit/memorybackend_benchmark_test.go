package ratelimit

import (
	"crypto/sha256"
	"fmt"
	"github.com/9edang/inkgate/context"
	"math/rand"
	"testing"
	"time"
)

func generateTestKeys(num, length int) []string {
	res := make([]string, 0, num)

	// we do not need crypto strength random number to test
	r := rand.New(rand.NewSource(int64(num))) // skipcq: GSC-G404
	h := sha256.New()

	for range num {
		fmt.Fprintf(h, "%x", r.Int())
		res = append(res, fmt.Sprintf("%x", h.Sum(nil))[:length])
	}
	return res
}

type buildCounter struct {
	counter int64
}

func (bc *buildCounter) builder() func() any {
	return func() any {
		bc.counter += 1
		return bc.counter
	}
}

func BenchmarkMemoryBackend(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	ttl := time.Millisecond

	maxNumToEvict := 1000000 // 1 million * (8 length + 4 ) ~ 12 megs ?
	maxNumToKeep := 1000000  //
	toEvict := generateTestKeys(maxNumToEvict, 8)
	toKeep := generateTestKeys(maxNumToKeep, 8)
	bc := buildCounter{}
	bcf := bc.builder()

	t := []struct {
		keep  int
		evict int
	}{
		{keep: 100, evict: 100},
		{keep: 100, evict: 10000},
		{keep: 100, evict: 1000000},
		{keep: 10000, evict: 100},
		{keep: 10000, evict: 10000},
		{keep: 10000, evict: 1000000},
	}

	for _, tc := range t {
		b.Run(fmt.Sprintf("loads_%d_keep_%d_evict_%d", tc.keep*10+tc.evict, tc.keep, tc.evict), func(b *testing.B) {
			mb := NewMemoryBackend(ctx, ttl)
			for b.Loop() {
				numMax := max(tc.evict, tc.keep)
				// load all one time
				for k := range numMax {
					if k < tc.keep {
						mb.Load(toKeep[k], bcf)
					}
					if k < tc.evict {
						mb.Load(toEvict[k], bcf)
					}
				}
				// now keep loading the to keep 9 times
				// it might or might not evict depending on the time it takes
				for range 9 {
					for k := range tc.keep {
						mb.Load(toKeep[k], bcf)
					}
				}
			}
		})
	}
	cancel()
}
