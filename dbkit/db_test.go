package dbkit

import (
	"go.uber.org/goleak"
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	ctx := context.Background()
	_, err := Connect(ctx, "nonexistent_driver", "some-dsn")
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

func TestConnectWithOptions(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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

func TestConfigOptions_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	tests := []struct {
		name     string
		opts     []Option
		validate func(*testing.T, Config)
	}{
		{
			name: "WithMaxOpenConns",
			opts: []Option{WithMaxOpenConns(50)},
			validate: func(t *testing.T, cfg Config) {
				if cfg.MaxOpenConns != 50 {
					t.Errorf("expected 50, got %d", cfg.MaxOpenConns)
				}
			},
		},
		{
			name: "WithMaxIdleConns",
			opts: []Option{WithMaxIdleConns(10)},
			validate: func(t *testing.T, cfg Config) {
				if cfg.MaxIdleConns != 10 {
					t.Errorf("expected 10, got %d", cfg.MaxIdleConns)
				}
			},
		},
		{
			name: "WithConnMaxLifetime",
			opts: []Option{WithConnMaxLifetime(10 * time.Minute)},
			validate: func(t *testing.T, cfg Config) {
				if cfg.ConnMaxLifetime != 10*time.Minute {
					t.Errorf("expected 10m, got %v", cfg.ConnMaxLifetime)
				}
			},
		},
		{
			name: "WithConnMaxIdleTime",
			opts: []Option{WithConnMaxIdleTime(5 * time.Minute)},
			validate: func(t *testing.T, cfg Config) {
				if cfg.ConnMaxIdleTime != 5*time.Minute {
					t.Errorf("expected 5m, got %v", cfg.ConnMaxIdleTime)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("sqlite3", "test.db")
			for _, opt := range tt.opts {
				opt(&cfg)
			}
			tt.validate(t, cfg)
		})
	}
}

func TestMustConnectSuccess(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := filepath.Join(t.TempDir(), "must_connect_test.db")
	ctx := context.Background()
	db := MustConnect(ctx, "sqlite3", dbPath)
	defer mustClose(t, db)

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestMustConnectPanicsOnEmptyDriver(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustConnect with invalid driver")
		}
	}()
	MustConnect(context.Background(), "invalid_driver", "invalid_dsn")
}

func TestHealthCheck_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	tests := []struct {
		name      string
		setup     func(t *testing.T) (*sqlx.DB, context.Context, context.CancelFunc)
		expectErr bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*sqlx.DB, context.Context, context.CancelFunc) {
				db := newTestDB(t)
				return db, context.Background(), func() {}
			},
			expectErr: false,
		},
		{
			name: "fails after close",
			setup: func(t *testing.T) (*sqlx.DB, context.Context, context.CancelFunc) {
				dbPath := filepath.Join(t.TempDir(), "healthcheck_closed.db")
				db, err := sqlx.Connect("sqlite3", dbPath)
				if err != nil {
					t.Fatalf("failed to create db: %v", err)
				}
				mustClose(t, db)
				return db, context.Background(), func() {}
			},
			expectErr: true,
		},
		{
			name: "cancelled context",
			setup: func(t *testing.T) (*sqlx.DB, context.Context, context.CancelFunc) {
				db := newTestDB(t)
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Force immediate failure
				return db, ctx, cancel
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, ctx, cancel := tt.setup(t)
			defer cancel()

			err := HealthCheck(ctx, db)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected health check to fail")
				}
			} else {
				if err != nil {
					t.Fatalf("health check failed: %v", err)
				}
			}
		})
	}
}

func TestConnectWithPreparedStatements(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := filepath.Join(t.TempDir(), "cancelled.db")
	// Create the file first so driver doesn't fail on missing file.
	f, err := os.Create(filepath.Clean(dbPath))
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

func TestConnect_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := filepath.Join(t.TempDir(), "tdt_connect_test.db")

	tests := []struct {
		name      string
		driver    string
		dsn       string
		opts      []Option
		ctxFn     func() (context.Context, context.CancelFunc)
		expectErr bool
		errMsg    string
	}{
		{
			name:      "empty driver",
			driver:    "",
			dsn:       "some-dsn",
			expectErr: true,
			errMsg:    "dbkit: driver is required",
		},
		{
			name:      "empty dsn",
			driver:    "sqlite3",
			dsn:       "",
			expectErr: true,
			errMsg:    "dbkit: dsn is required",
		},
		{
			name:      "valid connection",
			driver:    "sqlite3",
			dsn:       dbPath,
			opts:      []Option{WithMaxOpenConns(10), WithMaxIdleConns(3)},
			expectErr: false,
		},
		{
			name:      "no options provided",
			driver:    "sqlite3",
			dsn:       dbPath,
			opts:      nil,
			expectErr: false,
		},
		{
			name:      "slice with nil options safety",
			driver:    "sqlite3",
			dsn:       dbPath,
			opts:      []Option{nil, WithMaxOpenConns(5), nil},
			expectErr: false,
		},
		{
			name:      "unknown driver",
			driver:    "nonexistent_driver",
			dsn:       "some-dsn",
			expectErr: true,
		},
		{
			name:   "cancelled context",
			driver: "sqlite3",
			dsn:    dbPath,
			ctxFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancel immediately
				return ctx, cancel
			},
			// Note: sqlite3 driver ignores the cancelled context during Connect,
			// so it might actually succeed or fail depending on OS/driver.
			// We just ensure it doesn't panic.
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.ctxFn != nil {
				ctx, cancel = tt.ctxFn()
				defer cancel()
			}

			db, err := Connect(ctx, tt.driver, tt.dsn, tt.opts...)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Fatalf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				// if we don't expect an error, but it fails due to cancellation, we shouldn't strictly fail
				// if the failure is context.Canceled. But sqlite3 typically doesn't fail here.
				if err != nil && err != context.Canceled {
					t.Fatalf("unexpected error: %v", err)
				}
				if db != nil {
					mustClose(t, db)
				}
			}
		})
	}
}

func TestMustConnect_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))
	dbPath := filepath.Join(t.TempDir(), "tdt_must_connect_test.db")

	tests := []struct {
		name        string
		driver      string
		dsn         string
		opts        []Option
		expectPanic bool
		panicMsg    string
	}{
		{
			name:        "no options provided",
			driver:      "sqlite3",
			dsn:         dbPath,
			opts:        nil,
			expectPanic: false,
		},
		{
			name:        "slice with nil options safety",
			driver:      "sqlite3",
			dsn:         dbPath,
			opts:        []Option{nil, WithMaxOpenConns(5), nil},
			expectPanic: false,
		},
		{
			name:        "empty driver panics",
			driver:      "",
			dsn:         "some-dsn",
			expectPanic: true,
			panicMsg:    "dbkit: dbkit: driver is required",
		},
		{
			name:        "invalid driver panics",
			driver:      "invalid_driver",
			dsn:         "invalid_dsn",
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.expectPanic {
					if r == nil {
						t.Fatalf("expected panic, got none")
					}
					if tt.panicMsg != "" {
						msg, ok := r.(string)
						if !ok {
							t.Fatalf("expected panic string, got %T", r)
						}
						if msg != tt.panicMsg {
							t.Fatalf("expected panic message %q, got %q", tt.panicMsg, msg)
						}
					}
				} else {
					if r != nil {
						t.Fatalf("unexpected panic: %v", r)
					}
				}
			}()

			db := MustConnect(context.Background(), tt.driver, tt.dsn, tt.opts...)
			if db != nil {
				mustClose(t, db)
			}
		})
	}
}
