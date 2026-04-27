// Package logkit provides structured JSON logging capabilities using the standard
// library's log/slog package. It offers a simple configuration API to easily
// initialize thread-safe, JSON-formatted loggers suitable for production environments.
package logkit

import (
	"io"
	"log/slog"
	"os"
)

// Config holds the configuration options necessary for initializing a new
// structured slog.Logger instance.
//
// Purpose: Configuration payload.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutative during setup, read-only afterwards.
type Config struct {
	Level  slog.Level
	Writer io.Writer
}

// Option defines a functional option signature for configuring the logger
// initialization process.
//
// Purpose: Functional setup.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutative, to be executed synchronously.
type Option func(*Config)

// WithLevel dynamically sets the minimum severity level for the logger
// (e.g., slog.LevelDebug, slog.LevelInfo). Logs below this level are discarded.
//
// Purpose: Alters log visibility.
// Constraints: None.
// Errors: None.
// Thread-safety: Synchronous struct mutation.
func WithLevel(level slog.Level) Option {
	return func(c *Config) {
		c.Level = level
	}
}

// WithWriter overrides the default output destination (os.Stdout) for the logger.
// Common targets include file handles, network sockets, or buffer streams.
//
// Purpose: Output management.
// Constraints: None.
// Errors: None.
// Thread-safety: Synchronous struct mutation.
func WithWriter(w io.Writer) Option {
	return func(c *Config) {
		c.Writer = w
	}
}

// NewLogger creates and returns an isolated, structured JSON logger initialized
// with the provided functional options.
//
// Purpose: Factory method.
// Constraints: It defaults to writing to os.Stdout at the Info level.
// Errors: None.
// Thread-safety: The returned slog.Logger instance securely synchronizes its own internal write state, making it inherently safe for concurrent use.
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

// Initialize instantiates a new logger and explicitly overwrites the global
// slog.Default() logger.
//
// Purpose: Sets up the global environment context.
// Constraints: This function mutates global application state and should typically only be called once during the application's bootstrap phase.
// Errors: None.
// Thread-safety: Modifying the global logger concurrently is generally safe as slog.SetDefault dynamically manages its own internal atomic pointer assignments.
func Initialize(opts ...Option) {
	logger := NewLogger(opts...)
	slog.SetDefault(logger)
}
