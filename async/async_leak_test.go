package async

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMap_NoGoroutineLeakOnContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	items := []int{1, 2, 3, 4, 5}

	workerStarted := make(chan struct{})

	go func() {
		<-workerStarted
		cancel()
	}()

	_, err := Map(ctx, items, 2, func(ctx context.Context, item int) (int, error) {
		if item == 1 {
			close(workerStarted)
		}
		// Block to simulate work and allow context cancellation to trigger
		time.Sleep(100 * time.Millisecond)
		return item * 2, nil
	})

	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}
