// Package cachekit provides a standardized caching module for GopherCore.
// Purpose: Offers a unified Cache interface.
// Constraints: Implementations must handle context cancellation.
// Thread-safety: Safe for concurrent use.

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
// Internal Logic Deep-Dive: The unified Cache interface deliberately standardizes on `[]byte` values for all read and write operations rather than generic interfaces (`any`). This architectural constraint ensures consistent serialization/deserialization behavior across both local memory boundaries and network boundaries (Redis), preventing runtime type assertion panics or subtle gob/JSON encoding discrepancies.
type Cache interface {
	// Set stores a value in the cache with the given key and expiration time.
	// If expiration is 0, the key has no expiration time.
	// Purpose: Allows persistent or volatile storage of byte slice data mapped by a string key.
	// Constraints: The key must not be empty. Context cancellation terminates external connections immediately.
	// Thread-safety: Implementations must guarantee thread-safety for concurrent writes.
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error

	// Get retrieves a value from the cache by its key.
	// Returns ErrCacheMiss if the key is not found.
	// Purpose: Fetches the cached byte slice associated with the specified key.
	// Constraints: Returns an exact match or ErrCacheMiss. Context cancellation is respected.
	// Thread-safety: Implementations must be safe for concurrent reads.
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key from the cache.
	// Does not return an error if the key does not exist.
	// Purpose: Removes a specific entry from the caching layer, freeing memory.
	// Constraints: Idempotent operation; deleting a non-existent key will not yield an error.
	// Thread-safety: Implementations must synchronize deletion alongside active reads and writes.
	Delete(ctx context.Context, key string) error
}
