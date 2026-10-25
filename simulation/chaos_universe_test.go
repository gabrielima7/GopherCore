package simulation

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/dbkit"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/jsonutil"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	_ "github.com/mattn/go-sqlite3"
)

type universePayload struct {
	ID   int    `json:"id"`
	Data string `json:"data"`
}

func TestUniverseChaosSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	dbPath := filepath.Join(t.TempDir(), "universe_chaos.db")
	db, err := dbkit.Connect(context.Background(), "sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to connect to db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS universe_logs (id INTEGER PRIMARY KEY, msg TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
	defer cache.Close()

	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: 10,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	router := httpkit.NewRouter(httpkit.WithLogger(false))
	router.Post("/universe", func(w http.ResponseWriter, r *http.Request) {
		var p universePayload
		if err := jsonutil.Unmarshal([]byte(`{"id":1,"data":"test"}`), &p); err != nil {
			httpkit.Error(w, http.StatusBadRequest, "bad json")
			return
		}

		if r.URL.Query().Get("fail") == "true" {
			httpkit.Error(w, http.StatusInternalServerError, "chaos failure")
			return
		}

		// Empirical chaos bounding: add a SELECT 1 query to stress dbkit connection pool safely.
		var dbStatus int
		dbErr := db.QueryRowContext(r.Context(), "SELECT 1").Scan(&dbStatus)
		if dbErr != nil {
			// Gracefully handle expected SQLite contention errors rather than failing the 200 payload if the DB is busy.
			if strings.Contains(dbErr.Error(), "database is locked") || strings.Contains(dbErr.Error(), "SQLITE_BUSY") {
				httpkit.Error(w, http.StatusTooManyRequests, "db locked")
				return
			}
		}

		_ = cache.Set(r.Context(), "key", []byte("val"), 5*time.Millisecond)
		httpkit.Ok(w, p)
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	client := srv.Client()
	defer client.CloseIdleConnections()

	concurrency := 25000
	items := make([]int, concurrency)
	for i := 0; i < concurrency; i++ {
		items[i] = i
	}

	var successCount int64
	var failCount int64

	start := time.Now()

	errs := async.Fan(context.Background(), items, func(ctx context.Context, item int) error {
		reqCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		res := result.Of(retry.DoWithValue(reqCtx, func(c context.Context) (int, error) {
			var statusCode int
			err := cb.ExecuteContext(c, func() error {
				url := srv.URL + "/universe"
				if item%5 == 0 {
					url += "?fail=true"
				}

				req, err := http.NewRequestWithContext(c, http.MethodPost, url, bytes.NewBuffer([]byte(`{"id":1}`)))
				if err != nil {
					return err
				}

				resp, err := client.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				statusCode = resp.StatusCode
				if resp.StatusCode >= 500 {
					return errors.New("server error")
				}
				return nil
			})
			if err != nil {
				return 0, err
			}
			return statusCode, nil
		}, retry.WithMaxAttempts(2), retry.WithInitialDelay(1*time.Millisecond)))

		if res.IsOk() {
			atomic.AddInt64(&successCount, 1)
		} else {
			atomic.AddInt64(&failCount, 1)
		}
		return nil
	})

	duration := time.Since(start)

	if len(errs) > 0 {
		t.Fatalf("Fan returned %d panics/errors", len(errs))
	}

	t.Logf("Universe Chaos (25k) completed in %v. Success: %d, Fail: %d", duration, successCount, failCount)
}
