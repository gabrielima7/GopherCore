// Package dbkit provides thread-safe database connection management, robust connection pooling defaults,
// and safe schema migration orchestration built upon sqlx and golang-migrate/migrate.
// Purpose: Act as the robust bridge for safe stateful persistence and schema evolution.
// Constraints: Target driver and DSN must be explicitly configured.
// Thread-safety: Uses sqlx connection pools internally, rendering database operations completely thread-safe.
// Internal Logic Deep-Dive: We aggressively enforce MaxIdleConns and MaxOpenConns dynamically at boot to prevent sudden thundering herd scenarios from exhausting connection limits on upstream database proxies like PgBouncer.
package dbkit
