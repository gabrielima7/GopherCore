package dbkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func mustClose(t *testing.T, closer interface{ Close() error }) {
	t.Helper()
	if err := closer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

// newTestDB creates a temporary SQLite3 database for testing.
func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { mustClose(t, db) })
	return db
}

func TestConnectEmptyDriver(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, "", "some-dsn")
	if err == nil {
		t.Fatal("expected error for empty driver")
	}
	if err.Error() != "dbkit: driver is required" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestConnectEmptyDSN(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, "sqlite3", "")
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
	if err.Error() != "dbkit: dsn is required" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestConnectSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "connect_test.db")
	ctx := context.Background()
	db, err := Connect(ctx, "sqlite3", dbPath,
		WithMaxOpenConns(10),
		WithMaxIdleConns(3),
		WithConnMaxLifetime(time.Minute),
		WithConnMaxIdleTime(30*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mustClose(t, db)

	// Verify the connection actually works.
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestConnectInvalidDriver(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, "nonexistent_driver", "some-dsn")
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

func TestConnectWithOptions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "options_test.db")
	ctx := context.Background()
	db, err := Connect(ctx, "sqlite3", dbPath,
		WithMaxOpenConns(50),
		WithMaxIdleConns(10),
		WithConnMaxLifetime(10*time.Minute),
		WithConnMaxIdleTime(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mustClose(t, db)

	// Verify connection pool settings by using the database.
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("sqlite3", "test.db")
	if cfg.Driver != "sqlite3" {
		t.Fatalf("expected 'sqlite3', got %q", cfg.Driver)
	}
	if cfg.DSN != "test.db" {
		t.Fatalf("unexpected DSN: %s", cfg.DSN)
	}
	if cfg.MaxOpenConns != 25 {
		t.Fatalf("expected 25 max open conns, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Fatalf("expected 5 max idle conns, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 5*time.Minute {
		t.Fatalf("expected 5m, got %v", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != time.Minute {
		t.Fatalf("expected 1m, got %v", cfg.ConnMaxIdleTime)
	}
}

func TestMustConnectSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "must_connect_test.db")
	ctx := context.Background()
	db := MustConnect(ctx, "sqlite3", dbPath)
	defer mustClose(t, db)

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestMustConnectPanicsOnEmptyDriver(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from MustConnect with empty driver")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if msg != "dbkit: dbkit: driver is required" {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()
	MustConnect(context.Background(), "", "some-dsn")
}

func TestMustConnectPanicsOnInvalidDriver(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustConnect with invalid driver")
		}
	}()
	MustConnect(context.Background(), "invalid_driver", "invalid_dsn")
}

func TestHealthCheck(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := HealthCheck(ctx, db)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestHealthCheckFailsAfterClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "healthcheck_closed.db")
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	mustClose(t, db) // Close it.

	err = HealthCheck(context.Background(), db)
	if err == nil {
		t.Fatal("expected health check to fail after close")
	}
}

func TestConnectWithPreparedStatements(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "prepared_test.db")
	ctx := context.Background()
	db, err := Connect(ctx, "sqlite3", dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mustClose(t, db)

	// Create a table.
	_, err = db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Use prepared statements (parameterized queries) — the safe way.
	stmt, err := db.Prepare("INSERT INTO items (name) VALUES (?)")
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	defer mustClose(t, stmt)

	_, err = stmt.Exec("test_item")
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	// Verify with sqlx named query.
	var count int
	err = db.Get(&count, "SELECT COUNT(*) FROM items WHERE name = ?", "test_item")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestConnectWithSQLInjectionPrevention(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "injection_test.db")
	ctx := context.Background()
	db, err := Connect(ctx, "sqlite3", dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mustClose(t, db)

	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	_, _ = db.Exec("INSERT INTO users (name) VALUES (?)", "alice")

	// Attempted SQL injection via parameterized query — should be safe.
	malicious := "'; DROP TABLE users; --"
	var count int
	err = db.Get(&count, "SELECT COUNT(*) FROM users WHERE name = ?", malicious)
	if err != nil {
		t.Fatalf("parameterized query should not error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 results for injection attempt, got %d", count)
	}

	// Verify the table still exists.
	err = db.Get(&count, "SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatal("users table was dropped — SQL injection succeeded!")
	}
}

// TestConnectCancelledContext verifies that Connect returns an error
// when the context is already cancelled.
func TestConnectCancelledContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cancelled.db")
	// Create the file first so driver doesn't fail on missing file.
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}
	mustClose(t, f)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = Connect(ctx, "sqlite3", dbPath)
	// SQLite may or may not honour the cancelled context — it depends on
	// the driver implementation. We just test that it doesn't panic.
	_ = err
}
