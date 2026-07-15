package cachekit

import (
	"context"
	"sync"
	"time"
)

// cacheItem represents a single item in the in-memory cache.
type cacheItem struct {
	value      []byte
	expiration time.Time
}

// InMemoryCache is a local, thread-safe, in-memory Cache implementation.
// Purpose: Provides fast, local caching without external dependencies.
// Constraints: Memory-bound. Does not automatically evict items based on memory pressure.
// Thread-safety: Safe for concurrent use, synchronized via sync.RWMutex.
type InMemoryCache struct {
	mu        sync.RWMutex
	items     map[string]cacheItem
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewInMemoryCache creates a new InMemoryCache instance and starts a background cleanup routine.
// The cleanupInterval dictates how often expired items are actively purged.
// Purpose: Initializes a local in-memory cache.
// Constraints: Callers should call Close() to release background resources when done.
// Thread-safety: Returns a thread-safe InMemoryCache.
func NewInMemoryCache(cleanupInterval time.Duration) *InMemoryCache {
	c := &InMemoryCache{
		items:  make(map[string]cacheItem),
		stopCh: make(chan struct{}),
	}

	if cleanupInterval > 0 {
		c.wg.Add(1)
		go c.cleanupLoop(cleanupInterval)
	}

	return c
}

// cleanupLoop periodically removes expired items from the cache.
func (c *InMemoryCache) cleanupLoop(interval time.Duration) {
	defer c.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopCh:
			return
		}
	}
}

// evictExpired removes all expired items.
func (c *InMemoryCache) evictExpired() {
	now := time.Now()

	// Phase 1: O(N) Read-Only Scan.
	// We use RLock to prevent blocking active concurrent Get() and Set() requests
	// while we iterate over potentially millions of keys.
	c.mu.RLock()
	var expiredKeys []string
	for k, v := range c.items {
		if !v.expiration.IsZero() && now.After(v.expiration) {
			expiredKeys = append(expiredKeys, k)
		}
	}
	c.mu.RUnlock()

	if len(expiredKeys) == 0 {
		return
	}

	// Phase 2: O(E) Write-Lock Deletion (where E = number of expired keys).
	// We only acquire the exclusive Lock for the exact duration needed to delete,
	// mathematically minimizing lock contention and tail-latency spikes.
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range expiredKeys {
		// Internal Logic Deep-Dive: Double-check expiration. A high-concurrency Goroutine might have
		// updated this key with a fresh TTL between our RLock and Lock phases.
		if v, exists := c.items[k]; exists && !v.expiration.IsZero() && now.After(v.expiration) {
			delete(c.items, k)
		}
	}
}

// Close stops the background cleanup routine.
// Purpose: Frees resources associated with the background cleanup.
// Constraints: Must be called to prevent goroutine leaks.
// Thread-safety: Safe to call concurrently.
func (c *InMemoryCache) Close() error {
	c.closeOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
	return nil
}

// Set stores a value in the memory cache.
// Purpose: Implements Cache.Set using local map.
// Constraints: Context is checked for cancellation before operation.
// Thread-safety: Safe for concurrent use.
func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	}

	// Copy the value to ensure caller cannot modify it after setting
	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:      valCopy,
		expiration: exp,
	}

	return nil
}

// Get retrieves a value from the memory cache.
// Purpose: Implements Cache.Get using local map.
// Constraints: Context is checked for cancellation before operation. Returns ErrCacheMiss if not found or expired.
// Thread-safety: Safe for concurrent use.
func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	c.mu.RLock()
	item, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		return nil, ErrCacheMiss
	}

	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		// Item has expired, lazily delete it
		c.mu.Lock()
		// Internal Logic Deep-Dive: We double-check the expiration inside the write lock to prevent a race condition. Between releasing the `RLock` and acquiring the `Lock`, another goroutine could have aggressively updated the key with a fresh, non-expired value. If we didn't double-check `currentItem.expiration.Equal(item.expiration)`, we would accidentally delete perfectly valid new data.
		if currentItem, exists := c.items[key]; exists && currentItem.expiration.Equal(item.expiration) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil, ErrCacheMiss
	}

	// Copy the value to ensure caller cannot modify the cached value
	valCopy := make([]byte, len(item.value))
	copy(valCopy, item.value)

	return valCopy, nil
}

// Delete removes a key from the memory cache.
// Purpose: Implements Cache.Delete using local map.
// Constraints: Context is checked for cancellation before operation.
// Thread-safety: Safe for concurrent use.
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}
