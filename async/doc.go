// Package async provides safe goroutine management utilities including
// panic recovery, fan-out/fan-in patterns, and bounded concurrent mapping.
// All functions accept context.Context for cancellation support.
// Purpose: Enables zero-crash concurrent executions.
// Constraints: Assumes that panics can be captured gracefully.
// Thread-safety: Highly concurrent, safe for simultaneous invocations across unbounded goroutines.
package async
