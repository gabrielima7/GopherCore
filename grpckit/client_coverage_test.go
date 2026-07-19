package grpckit

import (
	"context"
	"testing"
	"time"
)

func TestNewClient_WithCustomContextDialer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	conn, _ := NewClient(ctx, "203.0.113.1:50051", WithInsecure(), WithDialTimeout(2*time.Millisecond))

	conn2, err := NewClient(ctx, "203.0.113.1:50051", WithInsecure())
	if err == nil {
		conn2.Connect()
		conn2.Close()
	}
	if conn != nil {
		conn.Close()
	}
}
