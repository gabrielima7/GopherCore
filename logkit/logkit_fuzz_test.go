package logkit

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func FuzzLogger(f *testing.F) {
	f.Add("Valid log message", "key1", []byte("value1"))
	f.Add("", "", []byte(""))
	f.Add("Garbage \x00\xff chars", "inv\x00lid", []byte("\xde\xad\xbe\xef"))

	f.Fuzz(func(t *testing.T, msg, key string, valBytes []byte) {
		var buf bytes.Buffer

		logger := NewLogger(WithWriter(&buf), WithLevel(slog.LevelDebug))

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Logger panicked with inputs: msg=%q key=%q valBytes=%q panic=%v", msg, key, valBytes, r)
			}
		}()

		logger.Debug(msg, slog.Any(key, valBytes))
		logger.InfoContext(context.Background(), msg, slog.String(key, string(valBytes)))
		logger.Warn(msg, slog.String(key, string(valBytes)))
		logger.Error(msg, slog.Any(key, string(valBytes)))
	})
}
