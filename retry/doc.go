// Package retry provides configurable retry logic with exponential backoff,
// jitter, and context-aware cancellation for fallible operations.
// Purpose: Orchestrates systematic re-executions for transiently failing network operations.
// Constraints: Dependent on accurate strategy configuration.
// Thread-safety: The engine handles independent isolated execution blocks concurrently.
package retry
