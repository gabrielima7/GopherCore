// Package dbkit provides thread-safe database connection management, robust connection pooling defaults,
// and safe schema migration orchestration built upon sqlx and golang-migrate/migrate.
package dbkit

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// Config holds database connection configuration.
//
// Purpose: Strongly typed definition of connection and pool requirements.
// Constraints: None.
// Errors: None.
// Thread-safety: Safely read-only post instantiation.
type Config struct {
	// Driver is the database driver name (e.g., "postgres", "mysql", "sqlite3").
	Driver string
	// DSN is the data source name / connection string.
	DSN string
	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum duration a connection can be reused.
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime is the maximum duration a connection can be idle.
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns a sensible default configuration
// mapped to a stable production-ready baseline.
//
// Purpose: Provide standardized pool settings for safety.
// Constraints: None.
// Errors: None.
// Thread-safety: Returns a new struct instance, inherently thread-safe.
func DefaultConfig(driver, dsn string) Config {
	return Config{
		Driver:          driver,
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// Option is a functional option for configuring the database connection mutatively.
//
// Purpose: Allows overriding default Config values.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutative, to be executed synchronously during initialization.
type Option func(*Config)

// WithMaxOpenConns sets the maximum number of open connections.
//
// Purpose: Overrides default max open connections.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutates configuration synchronously.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) {
		c.MaxOpenConns = n
	}
}

// WithMaxIdleConns sets the maximum number of idle connections.
//
// Purpose: Overrides default max idle connections.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutates configuration synchronously.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) {
		c.MaxIdleConns = n
	}
}

// WithConnMaxLifetime sets the maximum duration a connection can be reused.
//
// Purpose: Overrides default max connection lifetime.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutates configuration synchronously.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxLifetime = d
	}
}

// WithConnMaxIdleTime sets the maximum duration a connection can be idle.
//
// Purpose: Overrides default max connection idle time.
// Constraints: None.
// Errors: None.
// Thread-safety: Mutates configuration synchronously.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxIdleTime = d
	}
}

// Connect safely initializes and establishes a new, connection-pooled database connection
// using the provided driver and DSN.
//
// Purpose: Connects to the database and applies pool settings.
// Constraints: Driver and DSN cannot be empty. Fully respects the provided context for timeout/cancellation during connection and subsequent connectivity verification (PingContext).
// Errors: Fails if driver/DSN are empty, or if initial connection or ping fails.
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
		opt(&cfg)
	}

	db, err := sqlx.ConnectContext(ctx, cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return db, nil
}

// MustConnect acts exactly like Connect, but instead of returning an error, it deliberately panics
// if the connection or ping fails.
//
// Purpose: Aborts application startup instantly on DB failure.
// Constraints: Intended solely for application startup phases where the inability to reach the primary database is considered a fatal, unrecoverable state.
// Errors: Panics on failure.
// Thread-safety: Like Connect, the returned connection pool is inherently thread-safe.
func MustConnect(ctx context.Context, driver, dsn string, opts ...Option) *sqlx.DB {
	db, err := Connect(ctx, driver, dsn, opts...)
	if err != nil {
		panic("dbkit: " + err.Error())
	}
	return db
}

// HealthCheck executes a lightweight ping against the configured database to ensure the
// connection remains active and the underlying database is currently reachable.
//
// Purpose: Standardized readiness check.
// Constraints: Respects context timeouts and cancellations to prevent unbounded blocking.
// Errors: Fails if ping is unsuccessful.
// Thread-safety: Safe for concurrent use as the database connection pool internalizes locks.
func HealthCheck(ctx context.Context, db *sqlx.DB) error {
	return db.PingContext(ctx)
}
