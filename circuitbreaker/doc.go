// Package circuitbreaker provides the Circuit Breaker pattern to safely prevent
// cascading failures. It wraps fallible operations and trips when too many
// failures occur, allowing the system to recover gracefully.
// Purpose: Intercepts requests to backend services to prevent cascading failures during outages.
// Constraints: Operates based on configurable success and failure thresholds.
// Thread-safety: Relies on sync.Mutex, safe for concurrent use across goroutines.
package circuitbreaker
