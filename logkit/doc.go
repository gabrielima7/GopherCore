// Package logkit provides structured JSON logging capabilities using the standard
// library's log/slog package. It offers a simple configuration API to easily
// initialize thread-safe, JSON-formatted loggers suitable for production environments.
// Purpose: Centralized fast, structured JSON logging mechanism.
// Constraints: Intended to replace standard text-based loggers globally.
// Thread-safety: Logger methods implicitly lock outputs making them safe concurrently.
// Internal Logic Deep-Dive: The slog integration utilizes custom Append text buffers to format keys and values in-place, drastically cutting down on heap allocations per log line in high-throughput environments.
package logkit
