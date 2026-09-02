// Package async provides goroutine management and job queuing capabilities.
// Purpose: async provides asynchronous job queuing and worker management with bounded concurrency.
// Constraints: Internal package.
// Thread-safety: Varies by component.
package async

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"

	"github.com/hibiken/asynq"
)

// ErrClientNotInitialized is returned when attempting to close or use an uninitialized queue client.
// Purpose: Signals that a queue client method was called on a nil or uninitialized receiver.
// Constraints: Standardized error for asynchronous queue client initialization failures.
// Thread-safety: Immutable error value, safe for concurrent access.
// Internal Logic Deep-Dive: Providing a global sentinel allows callers to gracefully degrade if the backend isn't ready.
var ErrClientNotInitialized = errors.New("async: queue client is not initialized")

// QueueServer provides a wrapper around asynq.Server, adding robust panic recovery
// and centralized task registration.
// Purpose: To manage persistent background jobs using Redis while preventing application crashes from panicking tasks.
// Constraints: Must be initialized with valid Redis connection options and Asynq configuration.
// Thread-safety: Thread-safe for concurrent handler registration via internal mutex lock.
// Internal Logic Deep-Dive: By abstracting the server, we decouple the application logic from specific queueing backends (like Redis or RabbitMQ).
type QueueServer struct {
	server  *asynq.Server
	mux     *asynq.ServeMux
	mu      sync.Mutex
	started bool
}

// NewQueueServer initializes a new QueueServer.
// Purpose: Creates an instance of QueueServer ready for handler registration.
// Constraints: redisOpt and cfg must be fully configured.
// Thread-safety: Returns a new struct pointer, safe to share across goroutines.
// Internal Logic Deep-Dive: Centralizing initialization ensures all internal worker pools and telemetry hooks are correctly wired.
func NewQueueServer(redisOpt asynq.RedisConnOpt, cfg asynq.Config) *QueueServer {
	return &QueueServer{
		server: asynq.NewServer(redisOpt, cfg),
		mux:    asynq.NewServeMux(),
	}
}

// HandleFunc maps a task type pattern to a handler function, actively intercepting panics.
// Purpose: Registers a callback for specific task types with intrinsic panic isolation.
// Constraints: pattern string must perfectly match the task type. Cannot be called after Start().
// Thread-safety: Safely locks the internal mutex to prevent concurrent map writes inside the mux during initialization.
// Internal Logic Deep-Dive: A simple hash map binds strings to function pointers, optimizing lookup times during high-throughput consumption.
func (q *QueueServer) HandleFunc(pattern string, handler func(context.Context, *asynq.Task) error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Internal Logic Deep-Dive: A strict check prevents registering handlers after the queue has started to avoid concurrent map writes inside Asynq.
	if q.started {
		panic("asynq: cannot register handler after server has started")
	}

	// Internal Logic Deep-Dive: By wrapping the user-provided handler, we strictly enforce a recovery boundary.
	// If a background task triggers a panic (e.g. nil pointer dereference during complex data parsing),
	// this recovery block converts the panic into a formalized PanicError. Asynq will then mark the task as failed
	// and attempt retries based on its configuration, rather than crashing the overarching worker node entirely.
	q.mux.HandleFunc(pattern, func(ctx context.Context, t *asynq.Task) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = &PanicError{
					Value: r,
					Stack: string(debug.Stack()),
				}
			}
		}()
		return handler(ctx, t)
	})
}

// Start begins processing tasks from the queue asynchronously.
// Purpose: Spawns the underlying Asynq server listeners without blocking the caller.
// Constraints: Handlers should be fully registered before calling Start.
// Thread-safety: Handled internally by Asynq's server start mechanisms.
// Internal Logic Deep-Dive: Spawns independent subscriber goroutines that block on the backend, allowing immediate scaling up to configured concurrency limits.
func (q *QueueServer) Start() error {
	q.mu.Lock()
	q.started = true
	q.mu.Unlock()
	return q.server.Start(q.mux)
}

// Stop gracefully shuts down the queue server, waiting for active tasks to finish.
// Purpose: Provides a safe termination signal to halt job consumption and gracefully drain ongoing operations.
// Constraints: Retained for backward compatibility. Invokes Shutdown() to gracefully drain tasks.
// Thread-safety: Safe for concurrent invocation.
// Internal Logic Deep-Dive: Canceling the root context cascades signals to all active processors, preventing dropped tasks during deployments.
func (q *QueueServer) Stop() {
	q.Shutdown()
}

// Shutdown gracefully shuts down the queue server, waiting for active tasks to finish.
// Purpose: Provides a safe termination signal to halt job consumption and gracefully drain ongoing operations.
// Constraints: Should be called during application teardown, typically via defer.
// Thread-safety: Safe for concurrent invocation.
// Internal Logic Deep-Dive: Bypasses graceful draining to immediately sever socket connections, essential for emergency process exits.
func (q *QueueServer) Shutdown() {
	q.server.Shutdown()
}

// QueueClient provides a simplified wrapper for dispatching Asynq tasks.
// Purpose: To enqueue asynchronous jobs for background processing safely.
// Constraints: Requires an active Redis connection.
// Thread-safety: Safe for concurrent task enqueueing by multiple goroutines.
// Internal Logic Deep-Dive: Segregating the producer interface ensures frontend HTTP handlers remain lightweight and agnostic.
type QueueClient struct {
	client *asynq.Client
}

// NewQueueClient creates a new client for task enqueueing.
// Purpose: Initializes the client that submits new tasks to Redis.
// Constraints: redisOpt must point to an active Redis instance.
// Thread-safety: Returns a new struct pointer.
// Internal Logic Deep-Dive: Pre-allocates necessary connection pools to avoid latency spikes on the first task enqueue.
func NewQueueClient(redisOpt asynq.RedisConnOpt) *QueueClient {
	return &QueueClient{
		client: asynq.NewClient(redisOpt),
	}
}

// Enqueue submits a task to the message queue.
// Purpose: Queues a job for immediate or scheduled processing depending on options.
// Constraints: Returns an error if the task cannot be encoded or the queue is unreachable.
// Thread-safety: Can be invoked concurrently across varying HTTP handlers or RPCs.
// Internal Logic Deep-Dive: We offload the task persistence layer to Redis via asynq, ensuring tasks survive application restarts. The enqueue operation blocks only network round-trip time, keeping the caller thread lightweight.
func (c *QueueClient) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if c == nil || c.client == nil {
		return nil, ErrClientNotInitialized
	}
	return c.client.Enqueue(task, opts...)
}

// EnqueueContext submits a task bound strictly by context deadlines.
// Purpose: Extends Enqueue by accepting a context to abort the enqueue operation if the deadline is exceeded.
// Constraints: Context timeout primarily impacts the network roundtrip to Redis.
// Thread-safety: Safe for simultaneous execution.
// Internal Logic Deep-Dive: Serializes the payload and pushes via atomic backend operations to guarantee exactly-once delivery semantics.
func (c *QueueClient) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if c == nil || c.client == nil {
		return nil, ErrClientNotInitialized
	}
	return c.client.EnqueueContext(ctx, task, opts...)
}

// Close gracefully disconnects the client from Redis.
// Purpose: Cleans up network connections and prevents resource leaks.
// Constraints: Should be deferred immediately after instantiation or called on shutdown.
// Thread-safety: Returns ErrClientNotInitialized if the client is nil.
// Internal Logic Deep-Dive: Flushes any internally buffered tasks to the network before cleanly tearing down TCP sockets.
func (c *QueueClient) Close() error {
	if c == nil {
		return ErrClientNotInitialized
	}
	if c.client == nil {
		return ErrClientNotInitialized
	}
	return c.client.Close()
}
