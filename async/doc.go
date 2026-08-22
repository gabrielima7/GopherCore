// Package async provides safe goroutine management utilities including
// panic recovery, fan-out/fan-in patterns, and bounded concurrent mapping.
// All functions accept context.Context for cancellation support.
// Purpose: Enables zero-crash concurrent executions.
// Constraints: Assumes that panics can be captured gracefully.
// Thread-safety: Highly concurrent, safe for simultaneous invocations across unbounded goroutines.
// Internal Logic Deep-Dive: The package architecture leverages decoupled channels and sync primitives, utilizing blpop semantics under the hood to ensure low-latency task processing without burning CPU cycles on idle polling.
package async
