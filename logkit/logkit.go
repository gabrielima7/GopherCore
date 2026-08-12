// Package logkit provides utilities.
// Purpose: logkit provides a minimal, structured logging abstraction.
// Constraints: Internal package.
// Thread-safety: Varies by component.
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

// WithLevel sets the minimum severity threshold for emitting log records.
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

// NewLogger constructs the complete assembly and delivery of an entirely self-contained JSON-formatted observability engine customized precisely to the parameters supplied.
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
		if opt != nil {
			opt(&config)
		}
	}

	handler := slog.NewJSONHandler(config.Writer, &slog.HandlerOptions{
		Level: config.Level,
	})

	return slog.New(handler)
}

// Initialize configures the global slog logger with a structured JSON handler based on the provided options.
// Purpose: Bootstraps the application-wide logging engine.
// Constraints: This function mutates global application state and
// should typically only be called once during the application's bootstrap phase.
// Thread-safety: Modifying the global logger concurrently is generally safe as slog.SetDefault
// dynamically manages its own internal atomic pointer assignments.
func Initialize(opts ...Option) {
	logger := NewLogger(opts...)
	// Internal Logic Deep-Dive: slog.SetDefault safely updates the internal pointer atomically. This means active goroutines logging concurrently during this exact application-wide initialization call will seamlessly transition to the new JSON-formatted logger without experiencing a race condition or requiring an external global mutex lock.
	slog.SetDefault(logger)
}
