package cachekit_test

import (
	"context"
	"fmt"
	"go.uber.org/goleak"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/cachekit"
)

func TestInMemoryCache_EstimatedExpired(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name       string
		numItems   int
		expiration time.Duration
		sleepDur   time.Duration
	}{
		{
			name:       "estimatedExpired less than 16",
			numItems:   100,
			expiration: 5 * time.Millisecond,
			sleepDur:   30 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
			defer func() { _ = cache.Close() }()

			ctx := context.Background()
			for i := 0; i < tt.numItems; i++ {
				key := fmt.Sprintf("key_%d", i)
				err := cache.Set(ctx, key, []byte("val"), tt.expiration)
				if err != nil {
					t.Fatalf("Set failed: %v", err)
				}
			}

			// wait for cleanupLoop to trigger evictExpired
			time.Sleep(tt.sleepDur)

			// ensure they are gone
			for i := 0; i < tt.numItems; i++ {
				key := fmt.Sprintf("key_%d", i)
				_, err := cache.Get(ctx, key)
				if err == nil {
					t.Errorf("expected %s to be evicted", key)
				}
			}
		})
	}
}
