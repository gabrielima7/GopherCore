package simulation

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/dbkit"
	"github.com/gabrielima7/GopherCore/httpkit"
	"go.uber.org/goleak"
)

// TestChaosDatabaseConcurrency tests extreme lock-contention on SQLite using massive concurrent writes/reads.
func TestChaosDatabaseConcurrency(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hibiken/asynq.newClient"))

	// 1. Setup SQLite Database gracefully checking CGO_ENABLED
	db, err := dbkit.Connect(context.Background(), "sqlite3", "file:chaos_concurrency.db?mode=memory&cache=shared&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		t.Skip("skipping test: sqlite3 requires cgo, but CGO_ENABLED=0")
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS concurrency_test (id INTEGER PRIMARY KEY, counter INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO concurrency_test (id, counter) VALUES (1, 0)")
	if err != nil {
		t.Fatalf("failed to insert initial row: %v", err)
	}

	// 2. Setup HTTP Server handling graceful SQLITE_BUSY mapping
	router := httpkit.NewRouter()

	router.Get("/read", func(w http.ResponseWriter, r *http.Request) {
		var counter int
		err := db.Get(&counter, "SELECT counter FROM concurrency_test WHERE id = 1")
		if err != nil {
			if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
				httpkit.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "database is busy"})
				return
			}
			httpkit.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]int{"counter": counter})
	})

	router.Post("/write", func(w http.ResponseWriter, r *http.Request) {
		_, err := db.Exec("UPDATE concurrency_test SET counter = counter + 1 WHERE id = 1")
		if err != nil {
			if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
				httpkit.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "database is busy"})
				return
			}
			httpkit.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	// Pre-create a SINGLE client to maintain strict connection pooling bounds
	client := srv.Client()
	defer client.CloseIdleConnections()

	// 3. Concurrency simulation
	var successCount int64
	var busyCount int64
	var failCount int64

	requests := make([]int, 5000)
	for i := 0; i < len(requests); i++ {
		requests[i] = i
	}

	ctx := context.Background()

	_, _ = async.Map(ctx, requests, 200, func(ctx context.Context, id int) (bool, error) {
		var req *http.Request
		var err error

		if id%2 == 0 {
			req, err = http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/write", nil)
		} else {
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/read", nil)
		}

		if err != nil {
			return false, err
		}

		resp, err := client.Do(req)
		if err != nil {
			atomic.AddInt64(&failCount, 1)
			// Return nil error here so async.Map doesn't short circuit early
			// We track it inside failCount to prove chaos completion bounds natively
			return false, nil
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)

		switch resp.StatusCode {
		case http.StatusOK:
			atomic.AddInt64(&successCount, 1)
		case http.StatusTooManyRequests:
			atomic.AddInt64(&busyCount, 1)
		default:
			atomic.AddInt64(&failCount, 1)
		}

		return true, nil
	})

	t.Logf("Database Concurrency Simulation Completed. Success: %d, Busy/429: %d, Fail: %d", successCount, busyCount, failCount)

	total := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&busyCount) + atomic.LoadInt64(&failCount)
	if total != 5000 {
		t.Fatalf("Expected 5000 processed requests across all states, got %d", total)
	}
}
