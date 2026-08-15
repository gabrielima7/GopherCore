// Package cachekit provides utilities.
// Purpose: cachekit offers in-memory and Redis-backed caching mechanisms with strict serialization rules.
// Constraints: Internal package.
// Thread-safety: Varies by component.
package cachekit

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a Cache implementation using Redis.
// Purpose: Provides distributed, high-performance caching.
// Constraints: Requires a running Redis server and valid go-redis client.
// Thread-safety: Safe for concurrent use, backed by go-redis thread-safe client.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new RedisCache instance.
// Purpose: Initializes a Redis-backed cache.
// Constraints: The provided redis.Client must be properly configured and connected.
// Thread-safety: Returns a thread-safe RedisCache.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
	}
}

// Set stores a value in the Redis cache.
// Purpose: Implements Cache.Set using Redis SET command.
// Constraints: Context is respected for the operation timeout.
// Thread-safety: Safe for concurrent use.
// Internal Logic Deep-Dive: We map directly to the `go-redis` `Set` method which transparently handles executing the underlying Redis `SET` command. The duration parameter maps safely to Redis' native `PX` expiration capabilities without custom logic.
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	err := c.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

// Get retrieves a value from the Redis cache.
// Purpose: Implements Cache.Get using Redis GET command.
// Constraints: Returns ErrCacheMiss if the key does not exist.
// Thread-safety: Safe for concurrent use.
// Internal Logic Deep-Dive: When the underlying `go-redis` library does not find a key, it returns a specific `redis.Nil` error. We explicitly intercept this and remap it to our domain-specific `ErrCacheMiss`. This ensures that callers depending on the unified `Cache` interface are not tightly coupled to external Redis implementation details, preventing abstraction leakage.
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	return val, nil
}

// Delete removes a key from the Redis cache.
// Purpose: Implements Cache.Delete using Redis DEL command.
// Constraints: Does not error if the key doesn't exist.
// Thread-safety: Safe for concurrent use.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	return nil
}
