package logkit

import (
	"io"
	"log/slog"
	"os"
)

// Config holds the configuration options necessary for initializing a new
// structured slog.Logger instance.
type Config struct {
	Level  slog.Level
	Writer io.Writer
}

// Option defines a functional option signature for configuring the logger.
type Option func(*Config)

// WithLevel dynamically sets the minimum severity level for the logger
// (e.g., slog.LevelDebug, slog.LevelInfo). Logs below this level are discarded.
func WithLevel(level slog.Level) Option {
	return func(c *Config) {
		c.Level = level
	}
}

// WithWriter overrides the default output destination (os.Stdout) for the logger.
// Common targets include file handles, network sockets, or buffer streams.
func WithWriter(w io.Writer) Option {
	return func(c *Config) {
		c.Writer = w
	}
}

// NewLogger creates and returns an isolated, structured JSON logger initialized
// with the provided functional options. It defaults to writing to os.Stdout
// at the Info level. The returned logger is safe for concurrent use.
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
// slog.Default() logger. This function mutates global application state and
// should typically only be called once during the application's bootstrap phase.
func Initialize(opts ...Option) {
	logger := NewLogger(opts...)
	slog.SetDefault(logger)
}
