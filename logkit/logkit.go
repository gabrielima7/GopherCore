package logkit

import (
	"io"
	"log/slog"
	"os"
)

// Config holds the configuration for the structured logger.
type Config struct {
	Level  slog.Level
	Writer io.Writer
}

// Option represents a configuration option for the logger.
type Option func(*Config)

// WithLevel sets the minimum log level.
func WithLevel(level slog.Level) Option {
	return func(c *Config) {
		c.Level = level
	}
}

// WithWriter sets the output writer for the logger.
func WithWriter(w io.Writer) Option {
	return func(c *Config) {
		c.Writer = w
	}
}

// NewLogger creates a new structured JSON logger with the provided options.
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

// Initialize sets the default global logger to a structured JSON logger.
func Initialize(opts ...Option) {
	logger := NewLogger(opts...)
	slog.SetDefault(logger)
}
