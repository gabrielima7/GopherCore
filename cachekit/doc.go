// Package cachekit provides a standardized caching module for GopherCore.
//
// Purpose:
// It offers a unified Cache interface with implementations for both
// high-performance distributed caching (Redis) and local in-memory caching.
//
// Constraints:
// Implementations must handle context cancellation, respect TTLs,
// and handle cache misses gracefully using domain-specific errors.
//
// Thread-safety:
// All implementations of the Cache interface are safe for concurrent use
// by multiple goroutines.
//
// Internal Logic Deep-Dive:
// This package uses a unified `Cache` interface to allow applications to seamlessly switch
// between an external Redis store (`RedisCache`) for distributed environments, and a custom
// `sync.RWMutex`-backed implementation (`MemoryCache`) for local, zero-dependency deployments
// or fallback scenarios without requiring application code changes.
package cachekit
