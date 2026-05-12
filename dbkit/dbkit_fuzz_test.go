package dbkit

import (
	"context"
	"testing"
)

func FuzzConnect(f *testing.F) {
	f.Add("postgres", "user=foo password=bar dbname=test sslmode=disable")
	f.Add("mysql", "user:password@tcp(localhost:5555)/dbname")
	f.Add("sqlite3", "file::memory:?cache=shared")
	f.Add("", "")
	f.Add("postgres", "")
	f.Add("", "user=foo")

	f.Fuzz(func(t *testing.T, driver, dsn string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Connect panicked: driver=%q dsn=%q panic=%v", driver, dsn, r)
			}
		}()

		db, err := Connect(context.Background(), driver, dsn)
		if db != nil {
			_ = db.Close()
		}
		_ = err
	})
}
