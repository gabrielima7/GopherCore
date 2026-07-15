package cachekit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/redis/go-redis/v9"
)

func TestRedisCache(t *testing.T) {
	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = client.Close() }()

	cache := cachekit.NewRedisCache(client)
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "SetAndGet",
			run: func(t *testing.T) {
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
				_, err := cache.Get(ctx, "nonexistent")
				if !errors.Is(err, cachekit.ErrCacheMiss) {
					t.Errorf("expected ErrCacheMiss, got %v", err)
				}
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) {
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
			name: "Expiration",
			run: func(t *testing.T) {
				err := cache.Set(ctx, "key_exp", []byte("val"), 100*time.Millisecond)
				if err != nil {
					t.Fatalf("Set failed: %v", err)
				}

				mr.FastForward(200 * time.Millisecond)

				_, err = cache.Get(ctx, "key_exp")
				if !errors.Is(err, cachekit.ErrCacheMiss) {
					t.Errorf("expected ErrCacheMiss after expiration, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
