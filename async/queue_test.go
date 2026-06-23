package async

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
)

func TestQueueLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisOpt := asynq.RedisClientOpt{Addr: mr.Addr()}

	client := NewQueueClient(redisOpt)
	defer client.Close()

	srv := NewQueueServer(redisOpt, asynq.Config{
		Concurrency: 1,
		Queues: map[string]int{
			"default": 1,
		},
	})

	taskExecuted := make(chan bool, 1)

	srv.HandleFunc("test:task", func(ctx context.Context, task *asynq.Task) error {
		taskExecuted <- true
		return nil
	})

	err = srv.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	task := asynq.NewTask("test:task", nil)
	_, err = client.Enqueue(task)
	if err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-taskExecuted:
		// success
	case <-timer.C:
		t.Fatal("timeout waiting for task execution")
	}
}

func TestQueuePanicRecovery(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisOpt := asynq.RedisClientOpt{Addr: mr.Addr()}

	client := NewQueueClient(redisOpt)
	defer client.Close()

	srv := NewQueueServer(redisOpt, asynq.Config{
		Concurrency: 1,
		Queues: map[string]int{
			"default": 1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			var pe *PanicError
			if !errors.As(err, &pe) {
				t.Errorf("expected PanicError, got %T: %v", err, err)
			}
		}),
	})

	taskExecuted := make(chan struct{})

	srv.HandleFunc("test:panic", func(ctx context.Context, task *asynq.Task) error {
		close(taskExecuted)
		panic("task panic")
	})

	err = srv.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	task := asynq.NewTask("test:panic", nil)
	_, err = client.Enqueue(task)
	if err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-taskExecuted:
		// wait a bit for ErrorHandler to run
		time.Sleep(100 * time.Millisecond)
	case <-timer.C:
		t.Fatal("timeout waiting for task execution")
	}
}

func TestQueueClientEnqueueContext(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisOpt := asynq.RedisClientOpt{Addr: mr.Addr()}
	client := NewQueueClient(redisOpt)
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // instantly cancel to force failure

	task := asynq.NewTask("test:context", nil)
	_, err = client.EnqueueContext(ctx, task)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestQueueClientCloseUninitialized(t *testing.T) {
	var c *QueueClient
	err := c.Close()
	if !errors.Is(err, ErrClientNotInitialized) {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}

	c = &QueueClient{client: nil}
	err = c.Close()
	if !errors.Is(err, ErrClientNotInitialized) {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}
}

func TestQueueServerRegisterAfterStart(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisOpt := asynq.RedisClientOpt{Addr: mr.Addr()}
	srv := NewQueueServer(redisOpt, asynq.Config{})

	err = srv.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when registering handler after server start, got none")
		}
	}()

	srv.HandleFunc("test:post-start", func(ctx context.Context, task *asynq.Task) error {
		return nil
	})
}

func TestQueueClientEnqueueNil(t *testing.T) {
	var c *QueueClient
	task := asynq.NewTask("test:nil", nil)
	_, err := c.Enqueue(task)
	if !errors.Is(err, ErrClientNotInitialized) {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}

	c = &QueueClient{client: nil}
	_, err = c.Enqueue(task)
	if !errors.Is(err, ErrClientNotInitialized) {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}

	_, err = c.EnqueueContext(context.Background(), task)
	if !errors.Is(err, ErrClientNotInitialized) {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}

	var c2 *QueueClient
	_, err = c2.EnqueueContext(context.Background(), task)
	if !errors.Is(err, ErrClientNotInitialized) {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}
}
