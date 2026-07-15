package cachekit_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/redis/go-redis/v9"
)

func TestRedisCache_Errors(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer func() { _ = mr.Close() }()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = client.Close() }()

	cache := cachekit.NewRedisCache(client)

	// Create a canceled context to force errors from the Redis client
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "SetError_ContextCanceled",
			run: func(t *testing.T) {
				err := cache.Set(ctx, "key", []byte("val"), 0)
				if err == nil {
					t.Error("expected error due to canceled context, got nil")
				}
			},
		},
		{
			name: "GetError_ContextCanceled",
			run: func(t *testing.T) {
				_, err := cache.Get(ctx, "key")
				if err == nil {
					t.Error("expected error due to canceled context, got nil")
				}
			},
		},
		{
			name: "DeleteError_ContextCanceled",
			run: func(t *testing.T) {
				err := cache.Delete(ctx, "key")
				if err == nil {
					t.Error("expected error due to canceled context, got nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
