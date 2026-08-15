package logkit

import (
	"bytes"
	"encoding/json"
	"go.uber.org/goleak"
	"log/slog"
	"testing"
)

func TestNewLogger(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name          string
		opts          []Option
		useBuffer     bool
		logAction     func(logger *slog.Logger)
		expectedLines []map[string]interface{}
	}{
		{
			name:      "with debug level and writer",
			opts:      []Option{WithLevel(slog.LevelDebug)}, // Writer added dynamically
			useBuffer: true,
			logAction: func(logger *slog.Logger) {
				logger.Debug("debug message")
				logger.Info("info message")
			},
			expectedLines: []map[string]interface{}{
				{"level": "DEBUG", "msg": "debug message"},
				{"level": "INFO", "msg": "info message"},
			},
		},
		{
			name:      "no options provided",
			opts:      nil, // Strictly nil to test zero-options behavior
			useBuffer: false,
			logAction: func(logger *slog.Logger) {
				// We don't assert output since it writes to os.Stdout,
				// just verifying it doesn't panic.
				if logger == nil {
					t.Fatal("expected logger, got nil")
				}
			},
			expectedLines: nil,
		},
		{
			name:      "slice with nil options safety",
			opts:      []Option{nil, WithLevel(slog.LevelWarn), nil},
			useBuffer: true,
			logAction: func(logger *slog.Logger) {
				logger.Info("should not appear")
				logger.Warn("warn message")
			},
			expectedLines: []map[string]interface{}{
				{"level": "WARN", "msg": "warn message"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var logger *slog.Logger

			if tt.useBuffer {
				opts := append(tt.opts, WithWriter(&buf))
				logger = NewLogger(opts...)
			} else {
				logger = NewLogger(tt.opts...)
			}

			tt.logAction(logger)

			if !tt.useBuffer {
				return
			}

			for i, expected := range tt.expectedLines {
				line, err := buf.ReadBytes('\n')
				if err != nil {
					t.Fatalf("Failed to read log line %d: %v", i, err)
				}
				var logEntry map[string]interface{}
				if err := json.Unmarshal(line, &logEntry); err != nil {
					t.Fatalf("Failed to unmarshal log line %d: %v", i, err)
				}
				if logEntry["level"] != expected["level"] {
					t.Errorf("Expected level %v, got %v", expected["level"], logEntry["level"])
				}
				if logEntry["msg"] != expected["msg"] {
					t.Errorf("Expected msg %v, got %v", expected["msg"], logEntry["msg"])
				}
			}

			if buf.Len() > 0 {
				t.Errorf("Expected no more log lines, got: %s", buf.String())
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Capture the original logger to prevent global state leakage across tests.
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	tests := []struct {
		name          string
		opts          []Option
		useBuffer     bool
		logAction     func()
		expectedLines []map[string]interface{}
	}{
		{
			name:      "with warn level and writer",
			opts:      []Option{WithLevel(slog.LevelWarn)}, // Writer added dynamically
			useBuffer: true,
			logAction: func() {
				slog.Info("this should not be logged")
				slog.Warn("warning message")
			},
			expectedLines: []map[string]interface{}{
				{"level": "WARN", "msg": "warning message"},
			},
		},
		{
			name:      "no options provided",
			opts:      nil, // Strictly nil to test zero-options behavior
			useBuffer: false,
			logAction: func() {
				// We don't assert output since it writes to os.Stdout,
				// just verifying it doesn't panic.
				slog.Info("default info message")
			},
			expectedLines: nil,
		},
		{
			name:      "slice with nil options safety",
			opts:      []Option{nil, WithLevel(slog.LevelError), nil},
			useBuffer: true,
			logAction: func() {
				slog.Warn("should not appear")
				slog.Error("error message")
			},
			expectedLines: []map[string]interface{}{
				{"level": "ERROR", "msg": "error message"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			if tt.useBuffer {
				opts := append(tt.opts, WithWriter(&buf))
				Initialize(opts...)
			} else {
				Initialize(tt.opts...)
			}

			tt.logAction()

			if !tt.useBuffer {
				return
			}

			for i, expected := range tt.expectedLines {
				line, err := buf.ReadBytes('\n')
				if err != nil {
					t.Fatalf("Failed to read log line %d: %v", i, err)
				}
				var logEntry map[string]interface{}
				if err := json.Unmarshal(line, &logEntry); err != nil {
					t.Fatalf("Failed to unmarshal log line %d: %v", i, err)
				}
				if logEntry["level"] != expected["level"] {
					t.Errorf("Expected level %v, got %v", expected["level"], logEntry["level"])
				}
				if logEntry["msg"] != expected["msg"] {
					t.Errorf("Expected msg %v, got %v", expected["msg"], logEntry["msg"])
				}
			}

			if buf.Len() > 0 {
				t.Errorf("Expected no more log lines, got: %s", buf.String())
			}
		})
	}
}
