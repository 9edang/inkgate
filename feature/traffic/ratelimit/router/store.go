package router

import (
	"github.com/9edang/inkgate/context"

	limiter "github.com/9edang/inkgate/feature/traffic/ratelimit"
)

func StoreFromCfg(cfg Config) limiter.LimiterStore {
	ctx := context.Background()
	var storeBackend limiter.Backend
	if cfg.NumShards > 1 {
		storeBackend = limiter.NewShardedBackend(
			ctx,
			cfg.NumShards,
			cfg.TTL,
			cfg.CleanUpPeriod,
			1,
			limiter.PseudoFNV64a,
			limiter.MemoryBackendBuilder,
		)
	} else {
		storeBackend = limiter.MemoryBackendBuilder(ctx, cfg.TTL, cfg.CleanUpPeriod, 1, 1)[0]
	}

	// #nosec G115 -- numeric value from the operator config, far below the overflow range
	return limiter.NewLimiterStore(cfg.ClientMaxRate, int(cfg.ClientCapacity),
		storeBackend)
}
