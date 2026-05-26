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
package cachekit
