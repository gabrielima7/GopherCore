// Package dbkit provides thread-safe database connection management, robust connection pooling defaults,
// and safe schema migration orchestration built upon sqlx and golang-migrate/migrate.
// Purpose: Act as the robust bridge for safe stateful persistence and schema evolution.
// Constraints: Target driver and DSN must be explicitly configured.
// Thread-safety: Uses sqlx connection pools internally, rendering database operations completely thread-safe.
package dbkit

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// Config delineates the strict connectivity parameters and connection pooling thresholds used to dial the underlying remote database node.
// Purpose: Dictates the connection pool boundaries and driver settings.
// Constraints: Passed to connection handlers, typically not manually constructed.
// Thread-safety: Safely read-only post instantiation.
type Config struct {
	// Driver is the database driver name (e.g., "postgres", "mysql", "sqlite3").
	// Purpose: Specifies which underlying driver sqlx should initialize.
	// Constraints: Must be a registered database driver name.
	// Thread-safety: Read-only string.
	Driver string
	// DSN is the data source name / connection string.
	// Purpose: Contains connection credentials and routing information.
	// Constraints: Format strictly depends on the specified Driver.
	// Thread-safety: Read-only string.
	DSN string
	// MaxOpenConns is the maximum number of open connections.
	// Purpose: Throttles the maximum physical connections to the database.
	// Constraints: Must be >= 0.
	// Thread-safety: Read-only integer.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections.
	// Purpose: Caps the number of inactive connections kept alive in the pool.
	// Constraints: Must be >= 0.
	// Thread-safety: Read-only integer.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum duration a connection can be reused.
	// Purpose: Forces recycling of old connections to prevent stale socket issues.
	// Constraints: Must be >= 0.
	// Thread-safety: Read-only duration.
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime is the maximum duration a connection can be idle.
	// Purpose: Trims connections that have been inactive for too long.
	// Constraints: Must be >= 0.
	// Thread-safety: Read-only duration.
	ConnMaxIdleTime time.Duration
}

// DefaultConfig constructs a highly resilient network baseline, pre-populating safe connection thresholds that protect against arbitrary socket exhaustion in production.
// Purpose: Generates a baseline stable database connection configuration.
// Constraints: Assumes typical PostgreSQL/MySQL setups, might need tuning for highly constrained limits.
// Thread-safety: Returns a new value struct, safe to use across goroutines.
func DefaultConfig(driver, dsn string) Config {
	// These specific thresholds are chosen to prevent sudden bursts of traffic from
	// exhausting the upstream database's connection limits, balancing active capacity
	// against the memory overhead of maintaining stale idle sockets.
	return Config{
		Driver:          driver,
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// Option provides a composable configuration mutator, permitting callers to incrementally override bounded defaults prior to actively spinning up a database connection block.
// Purpose: Allows overriding default Config behavior.
// Constraints: Used as variadic arguments during Connect or MustConnect.
// Thread-safety: Safe when used sequentially during initialization.
type Option func(*Config)

// WithMaxOpenConns forcibly throttles the maximum volume of concurrent physical connections spawned across the entire connection pool lifecycle.
// Purpose: Adjusts the connection pool size limit.
// Constraints: n should be greater than 0.
// Thread-safety: Mutates configuration synchronously.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) {
		c.MaxOpenConns = n
	}
}

// WithMaxIdleConns dictates the exact ceiling of inactive sockets the host application will preserve in memory to accelerate upcoming network bursts.
// Purpose: Adjusts the connection pool idle limit.
// Constraints: n should be >= 0.
// Thread-safety: Mutates configuration synchronously.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) {
		c.MaxIdleConns = n
	}
}

// WithConnMaxLifetime actively caps the absolute chronological age of any connection inside the pool, severing stale transports that sit precariously close to typical database proxy drop timers.
// Purpose: Defines connection recycle limits.
// Constraints: Should be shorter than database-side closing timeouts.
// Thread-safety: Mutates configuration synchronously.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxLifetime = d
	}
}

// WithConnMaxIdleTime trims down the pool proactively by destroying cached connections if they idle inertly beyond this specified mathematical duration boundary.
// Purpose: Trims idle connections after this duration.
// Constraints: Must be >= 0.
// Thread-safety: Mutates configuration synchronously.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxIdleTime = d
	}
}

// Connect constructs and strictly calibrates a thread-safe connection pooling multiplexer over the specified database driver dialect, instantly validating network liveliness.
// Purpose: Opens a managed connection pool to a backing database system safely.
// Constraints: It fully respects the provided context for timeout/cancellation
// during connection and subsequent connectivity verification (PingContext).
// Thread-safety: The returned *sqlx.DB is inherently safe for concurrent access across multiple goroutines.
func Connect(ctx context.Context, driver, dsn string, opts ...Option) (*sqlx.DB, error) {
	if driver == "" {
		return nil, errors.New("dbkit: driver is required")
	}
	if dsn == "" {
		return nil, errors.New("dbkit: dsn is required")
	}

	cfg := DefaultConfig(driver, dsn)
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	// ConnectContext opens the database and then pings it. We rely on the provided context
	// to fail fast if the database is completely unreachable during application bootstrap.
	db, err := sqlx.ConnectContext(ctx, cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	// Enforce strict connection bounds to prevent connection starvation and database thrashing.
	// Bounding MaxOpenConns protects the downstream database from being overwhelmed, while
	// MaxIdleConns / MaxIdleTime ensures we aren't leaking stale sockets in memory.
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return db, nil
}

// MustConnect enforces standard connection bootstrapping, deliberately collapsing the entire runtime environment immediately upon experiencing any network or authentication friction.
// Purpose: Forces an immediate fatal panic if a connection fails, simplifying bootstrapping logic.
// Constraints: This is intended solely for application startup phases where
// the inability to reach the primary database is considered a fatal, unrecoverable state.
// Thread-safety: Like Connect, the returned connection pool is inherently thread-safe.
func MustConnect(ctx context.Context, driver, dsn string, opts ...Option) *sqlx.DB {
	// Execute standard connection bootstrapping, intentionally panicking on failure.
	// This is strictly designed for the application bootstrap phase where running
	// without a database connection leads to an unrecoverable zombie state.
	// Internal Logic Deep-Dive: Wrapping the Connect method directly minimizes code duplication. If the lower-level Connect fails due to malformed DSNs or offline engines, panic is triggered to halt orchestrator loops (e.g., Kubernetes) rather than risking a zombie process running without a stateful backend.
	db, err := Connect(ctx, driver, dsn, opts...)
	if err != nil {
		panic("dbkit: " + err.Error())
	}
	return db
}

// HealthCheck sends a rapid, lightweight verification packet directly to the underlying SQL driver engine to explicitly re-validate the current network socket stability state.
// Purpose: Assesses database liveliness dynamically.
// Constraints: It respects context timeouts and cancellations to prevent unbounded blocking.
// Thread-safety: Safe for concurrent use as the database connection pool internalizes locks.
func HealthCheck(ctx context.Context, db *sqlx.DB) error {
	return db.PingContext(ctx)
}
