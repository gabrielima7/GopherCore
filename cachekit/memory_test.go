package cachekit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/cachekit"
)

func TestInMemoryCache(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "SetAndGet",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0)
				defer cache.Close()

				err := cache.Set(ctx, "key1", []byte("val1"), 0)
				if err != nil {
					t.Fatalf("Set failed: %v", err)
				}

				val, err := cache.Get(ctx, "key1")
				if err != nil {
					t.Fatalf("Get failed: %v", err)
				}

				if string(val) != "val1" {
					t.Errorf("expected 'val1', got '%s'", string(val))
				}
			},
		},
		{
			name: "GetMiss",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0)
				defer cache.Close()

				_, err := cache.Get(ctx, "nonexistent")
				if !errors.Is(err, cachekit.ErrCacheMiss) {
					t.Errorf("expected ErrCacheMiss, got %v", err)
				}
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0)
				defer cache.Close()

				_ = cache.Set(ctx, "key_del", []byte("val"), 0)
				err := cache.Delete(ctx, "key_del")
				if err != nil {
					t.Fatalf("Delete failed: %v", err)
				}

				_, err = cache.Get(ctx, "key_del")
				if !errors.Is(err, cachekit.ErrCacheMiss) {
					t.Errorf("expected ErrCacheMiss after delete, got %v", err)
				}
			},
		},
		{
			name: "ExpirationLazy",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0) // No active cleanup
				defer cache.Close()

				err := cache.Set(ctx, "key_exp", []byte("val"), 10*time.Millisecond)
				if err != nil {
					t.Fatalf("Set failed: %v", err)
				}

				time.Sleep(20 * time.Millisecond)

				_, err = cache.Get(ctx, "key_exp")
				if !errors.Is(err, cachekit.ErrCacheMiss) {
					t.Errorf("expected ErrCacheMiss after expiration, got %v", err)
				}
			},
		},
		{
			name: "ExpirationActive",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
				defer cache.Close()

				err := cache.Set(ctx, "key_exp_active", []byte("val"), 20*time.Millisecond)
				if err != nil {
					t.Fatalf("Set failed: %v", err)
				}

				// Wait for expiration and active cleanup
				time.Sleep(50 * time.Millisecond)

				// Should return ErrCacheMiss naturally without active cleanup kicking in if we fetched,
				// but active cleanup ensures the memory is actually freed. We just test it misses.
				_, err = cache.Get(ctx, "key_exp_active")
				if !errors.Is(err, cachekit.ErrCacheMiss) {
					t.Errorf("expected ErrCacheMiss after active expiration, got %v", err)
				}
			},
		},
		{
			name: "Concurrency",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0)
				defer cache.Close()

				var wg sync.WaitGroup
				startCh := make(chan struct{})

				// Spawn readers and writers
				for i := 0; i < 50; i++ {
					wg.Add(2)

					// Writer
					go func(n int) {
						defer wg.Done()
						<-startCh
						_ = cache.Set(ctx, "shared_key", []byte("val"), 0)
					}(i)

					// Reader
					go func(n int) {
						defer wg.Done()
						<-startCh
						_, _ = cache.Get(ctx, "shared_key")
					}(i)
				}

				// Unblock all goroutines simultaneously
				close(startCh)
				wg.Wait()
			},
		},
		{
			name: "ContextCancellation",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0)
				defer cache.Close()

				canceledCtx, cancel := context.WithCancel(context.Background())
				cancel() // Pre-cancel

				err := cache.Set(canceledCtx, "k", []byte("v"), 0)
				if !errors.Is(err, context.Canceled) {
					t.Errorf("expected context.Canceled on Set, got %v", err)
				}

				_, err = cache.Get(canceledCtx, "k")
				if !errors.Is(err, context.Canceled) {
					t.Errorf("expected context.Canceled on Get, got %v", err)
				}

				err = cache.Delete(canceledCtx, "k")
				if !errors.Is(err, context.Canceled) {
					t.Errorf("expected context.Canceled on Delete, got %v", err)
				}
			},
		},
		{
			name: "IsolationCopy",
			run: func(t *testing.T) {
				cache := cachekit.NewInMemoryCache(0)
				defer cache.Close()

				orig := []byte("orig")
				_ = cache.Set(ctx, "k", orig, 0)

				// Mutate original slice
				orig[0] = 'm'

				val, _ := cache.Get(ctx, "k")
				if string(val) != "orig" {
					t.Errorf("cache value mutated! expected 'orig', got '%s'", string(val))
				}

				// Mutate fetched slice
				val[0] = 'x'

				val2, _ := cache.Get(ctx, "k")
				if string(val2) != "orig" {
					t.Errorf("cache value mutated via get! expected 'orig', got '%s'", string(val2))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
