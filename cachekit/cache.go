package cachekit

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss is returned when a key is not found in the cache.
// Purpose: Allows callers to differentiate between an actual error and a cache miss.
// Constraints: Standardized across all Cache implementations.
// Thread-safety: Immutable error value.
var ErrCacheMiss = errors.New("cache: key not found")

// Cache defines the standard interface for a caching layer.
// Purpose: Provides a unified API for both in-memory and distributed caches.
// Constraints: Implementations must be thread-safe and handle context cancellation.
// Thread-safety: Implementations must guarantee concurrent safety.
type Cache interface {
	// Set stores a value in the cache with the given key and expiration time.
	// If expiration is 0, the key has no expiration time.
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error

	// Get retrieves a value from the cache by its key.
	// Returns ErrCacheMiss if the key is not found.
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key from the cache.
	// Does not return an error if the key does not exist.
	Delete(ctx context.Context, key string) error
}
