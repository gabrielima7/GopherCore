package otelkit_test

import (
	"context"
	"testing"

	"github.com/gabrielima7/GopherCore/otelkit"
)

func TestInitSDK(t *testing.T) {
	ctx := context.Background()

	// Initialize the SDK with a test service name
	shutdown, err := otelkit.InitSDK(ctx, "test-service")

	if err != nil {
		t.Fatalf("expected no error during InitSDK, got %v", err)
	}
	if shutdown == nil {
		t.Fatalf("expected a valid shutdown function, got nil")
	}

	// Ensure shutdown completes without errors
	err = shutdown(ctx)
	if err != nil {
		t.Errorf("expected no error on shutdown, got %v", err)
	}
}
