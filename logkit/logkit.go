// Package logkit provides structured JSON logging capabilities using the standard
// library's log/slog package. It offers a simple configuration API to easily
// initialize thread-safe, JSON-formatted loggers suitable for production environments.
// Purpose: Centralized fast, structured JSON logging mechanism.
// Constraints: Intended to replace standard text-based loggers globally.
// Thread-safety: Logger methods implicitly lock outputs making them safe concurrently.
package logkit

import (
	"io"
	"log/slog"
	"os"
)

// Config dictates the fundamental runtime tuning dials required to securely bootstrap a new strictly typed, structured logging emitter.
// Purpose: Dictates logging levels and destinations.
// Constraints: Initialized indirectly via options.
// Thread-safety: Mutative during setup, read-only afterwards.
type Config struct {
	// Level determines the minimum severity threshold for emitting log records.
	// Purpose: Sets the noise threshold (e.g., Info vs Debug).
	// Constraints: Must be a valid slog.Level.
	// Thread-safety: Read-only during execution.
	Level slog.Level
	// Writer explicitly overrides the default logging output destination (os.Stdout) for capturing logs elsewhere.
	// Purpose: Directs log bytes to a specified sink.
	// Constraints: Must implement io.Writer and ideally handle concurrent writes safely.
	// Thread-safety: Read-only interface pointer.
	Writer io.Writer
}

// Option enforces a rigid functional type pattern, enabling developers to declaratively inject granular behavioral overrides when configuring the core application logger stack.
// Purpose: Allows overriding default log properties.
// Constraints: Functional mutators expected to be applied in order.
// Thread-safety: Safe when used sequentially during initialization.
type Option func(*Config)

// WithLevel calibrates the internal noise threshold filter, ruthlessly discarding any incoming logging messages that fall below the formally designated severity baseline.
// Purpose: Configure log verbosity.
// Constraints: Rejects logs that don't pass the check.
// Thread-safety: Synchronous struct mutation.
func WithLevel(level slog.Level) Option {
	return func(c *Config) {
		c.Level = level
	}
}

// WithWriter forcibly reroutes the physical byte stream destination away from the console and directly into an arbitrary provided I/O sink for advanced persistence or transport.
// Purpose: Maps the log output to a file or stream.
// Constraints: Assumes the writer is available.
// Thread-safety: Synchronous struct mutation.
func WithWriter(w io.Writer) Option {
	return func(c *Config) {
		c.Writer = w
	}
}

// NewLogger orchestrates the complete assembly and delivery of an entirely self-contained JSON-formatted observability engine customized precisely to the parameters supplied.
// Purpose: Instantiates a new independent slog logger.
// Constraints: It defaults to writing to os.Stdout at the Info level.
// Thread-safety: The returned slog.Logger instance securely synchronizes its own internal
// write state, making it inherently safe for concurrent use.
func NewLogger(opts ...Option) *slog.Logger {
	config := Config{
		Level:  slog.LevelInfo, // Default level
		Writer: os.Stdout,      // Default writer
	}

	for _, opt := range opts {
		opt(&config)
	}

	handler := slog.NewJSONHandler(config.Writer, &slog.HandlerOptions{
		Level: config.Level,
	})

	return slog.New(handler)
}

// Initialize aggressively overrides the underlying Go runtime's global standard logger entirely with a fresh, opinionated, structured instance bound to application-specific rules.
// Purpose: Bootstraps the application-wide logging engine.
// Constraints: This function mutates global application state and
// should typically only be called once during the application's bootstrap phase.
// Thread-safety: Modifying the global logger concurrently is generally safe as slog.SetDefault
// dynamically manages its own internal atomic pointer assignments.
func Initialize(opts ...Option) {
	logger := NewLogger(opts...)
	// slog.SetDefault safely updates the internal pointer, meaning active goroutines
	// logging concurrently during this call will seamlessly transition to the new instance.
	slog.SetDefault(logger)
}
