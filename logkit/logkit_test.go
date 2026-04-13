package logkit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(WithLevel(slog.LevelDebug), WithWriter(&buf))

	logger.Debug("debug message")
	logger.Info("info message")

	// Parse JSON lines from the buffer
	var logEntry map[string]interface{}

	// Read debug message
	line, err := buf.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read log line: %v", err)
	}
	if err := json.Unmarshal(line, &logEntry); err != nil {
		t.Fatalf("Failed to unmarshal log line: %v", err)
	}

	if logEntry["level"] != "DEBUG" {
		t.Errorf("Expected level DEBUG, got %v", logEntry["level"])
	}
	if logEntry["msg"] != "debug message" {
		t.Errorf("Expected msg 'debug message', got %v", logEntry["msg"])
	}

	// Read info message
	line, err = buf.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read log line: %v", err)
	}
	if err := json.Unmarshal(line, &logEntry); err != nil {
		t.Fatalf("Failed to unmarshal log line: %v", err)
	}

	if logEntry["level"] != "INFO" {
		t.Errorf("Expected level INFO, got %v", logEntry["level"])
	}
	if logEntry["msg"] != "info message" {
		t.Errorf("Expected msg 'info message', got %v", logEntry["msg"])
	}
}

func TestInitialize(t *testing.T) {
	var buf bytes.Buffer
	Initialize(WithLevel(slog.LevelWarn), WithWriter(&buf))

	slog.Info("this should not be logged")
	slog.Warn("warning message")

	line, err := buf.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read log line: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(line, &logEntry); err != nil {
		t.Fatalf("Failed to unmarshal log line: %v", err)
	}

	if logEntry["level"] != "WARN" {
		t.Errorf("Expected level WARN, got %v", logEntry["level"])
	}
	if logEntry["msg"] != "warning message" {
		t.Errorf("Expected msg 'warning message', got %v", logEntry["msg"])
	}

	// Make sure no more messages are logged
	if buf.Len() > 0 {
		t.Errorf("Expected no more log lines, got: %s", buf.String())
	}
}
