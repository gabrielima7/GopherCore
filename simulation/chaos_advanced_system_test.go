package simulation

import (
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
	"github.com/gabrielima7/GopherCore/retry"
	_ "github.com/mattn/go-sqlite3"
)

func TestAdvancedChaosSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreTopFunction("os/signal.NotifyContext.func1"))

	dbPath := filepath.Join(t.TempDir(), "advanced_chaos.db")
	db, err := dbkit.Connect(context.Background(), "sqlite3", dbPath)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("skipping test: sqlite3 requires cgo, but CGO_ENABLED=0")
		}
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS sys_metrics (id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000), // High rate limits
		httpkit.WithCORS("*"),
	)

	var requestCount int64
	router.Get("/advanced", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		failMode := r.URL.Query().Get("fail")
		switch failMode {
		case "panic":
			panic("simulated advanced panic")
		case "true":
			httpkit.Error(w, http.StatusInternalServerError, "simulated 500")
			return
		}

		var count int
		if err := db.Get(&count, "SELECT COUNT(*) FROM sys_metrics"); err != nil {
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}

		httpkit.JSON(w, http.StatusOK, map[string]interface{}{"msg": "advanced_success", "count": count})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()

	const numGoroutines = 5000
	group := async.NewGroup()
	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		idx := i
		group.Go(func() error {
			<-startSignal

			ctx := context.Background()
			var cancel context.CancelFunc
			if idx%50 == 0 {
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			} else if idx%100 == 0 {
				ctx, cancel = context.WithTimeout(ctx, 1*time.Millisecond)
				defer cancel()
			} else {
				ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
			}

			key := "advanced_key"
			if idx%3 == 0 {
				if _, err := cache.Get(ctx, key); err == nil {
					return nil
				}
			}

			_, err := retry.DoWithValue(ctx, func(c context.Context) (string, error) {
				var result string
				execErr := cb.ExecuteContext(ctx, func() error {
					endpoint := srv.URL + "/advanced"
					if idx%40 == 0 {
						endpoint += "?fail=panic"
					} else if idx%10 == 0 {
						endpoint += "?fail=true"
					}

					req, reqErr := http.NewRequestWithContext(c, "GET", endpoint, nil)
					if reqErr != nil {
						return reqErr
					}

					resp, doErr := client.Do(req)
					if doErr != nil {
						return doErr
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						result = "success"
						return nil
					}
					return errors.New("bad status")
				})
				return result, execErr
			}, retry.WithMaxAttempts(3), retry.WithInitialDelay(5*time.Millisecond))

			if err == nil {
				_ = cache.Set(ctx, key, []byte("success"), 5*time.Second)
			}
			return err
		})
	}

	close(startSignal)
	errs := group.Wait()

	for _, err := range errs {
		if err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "circuit is open") &&
				!strings.Contains(msg, "bad status") &&
				!strings.Contains(msg, "too many requests") &&
				!strings.Contains(msg, "connection refused") &&
				!strings.Contains(msg, "EOF") &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) {
				t.Errorf("Unexpected error during massive concurrency load: %v", err)
			}
		}
	}
}
