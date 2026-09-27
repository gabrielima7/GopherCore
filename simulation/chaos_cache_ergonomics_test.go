package simulation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/cachekit"
)

func TestCacheErgonomics_ExtremeEviction(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create cache with extremely fast cleanup tick (1ms) to maximize interference and eviction scans.
	cache := cachekit.NewInMemoryCache(1 * time.Millisecond)
	defer func() { _ = cache.Close() }()

	numGoroutines := 3000
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			key := fmt.Sprintf("key_%d", idx%100) // Collisions to ensure write-locks clash with RLock scans

			// 1. Set short TTL
			err := cache.Set(ctx, key, []byte("data"), 2*time.Millisecond)
			if err != nil {
				return
			}

			// 2. Immediate read
			_, _ = cache.Get(ctx, key)

			// 3. Sleep slightly to allow the 1ms TTL sweeper to kick in
			time.Sleep(3 * time.Millisecond)

			// 4. Read (expecting it might be missing or lazily expired)
			_, _ = cache.Get(ctx, key)

			// 5. Delete manually
			_ = cache.Delete(ctx, key)

		}(i)
	}

	close(startCh)
	wg.Wait()
}
