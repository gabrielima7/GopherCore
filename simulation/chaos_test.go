package simulation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/dbkit"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/jsonutil"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	_ "github.com/mattn/go-sqlite3"
)

type Payload struct {
	ID string `json:"id"`
}

func TestChaosMicroserviceSimulation(t *testing.T) {
	// Initialize Circuit Breaker
	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	// Initialize dbkit with SQLite
	dbPath := filepath.Join(t.TempDir(), "chaos_test.db")
	db := dbkit.MustConnect(context.Background(), "sqlite3", dbPath)
	defer db.Close()
	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	_, _ = db.Exec("INSERT INTO users (name) VALUES (?)", "alice")

	// Initialize Server Router
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(10000, 20000),
		httpkit.WithCORS("*"),
	)

	router.Post("/process", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			httpkit.Error(w, http.StatusBadRequest, "failed to read body")
			return
		}

		// Simulate unmarshal of untrusted payload
		var p Payload
		if err := jsonutil.Unmarshal(body, &p); err != nil {
			httpkit.Error(w, http.StatusBadRequest, "invalid payload")
			return
		}

		// Simulate success
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok", "id": p.ID})
	})

	router.Get("/db", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		var count int
		// Parametrized query avoids SQL injection
		err := db.Get(&count, "SELECT COUNT(*) FROM users WHERE name = ?", id)
		if err != nil {
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]int{"count": count})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	var wg sync.WaitGroup
	startCh := make(chan struct{})

	concurrencyLimit := 1000

	// Inject Concurrency Chaos
	for i := 0; i < concurrencyLimit; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-startCh // Block until start signal to ensure simultaneous execution

			// Simulate context cancellations
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if id%10 == 0 {
				cancel() // Force immediate cancellation for 10% of requests
			}

			// Test data varying per goroutine ID
			var isPost bool
			var endpoint string
			var payload []byte

			if id%2 == 0 {
				isPost = true
				endpoint = srv.URL + "/process"
				if id%4 == 0 {
					payload = []byte(`{"id": "test"}`)
				} else {
					// Malicious nested JSON payload
					payload = []byte(`{"id": "malicious", "nested": {"nested": {"nested": {"data": [1,2,3,4,5,6]}}}}`)
				}
			} else {
				isPost = false
				// SQL injection string
				endpoint = srv.URL + "/db?id=';+DROP+TABLE+users;+--"
			}

			// Integrate Retry, CircuitBreaker, Result, and HTTP
			res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
				var finalVal string
				err := cb.Execute(func() error {
					var req *http.Request
					var err error
					if isPost {
						req, err = http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
					} else {
						req, err = http.NewRequestWithContext(ctx, "GET", endpoint, nil)
					}

					if err != nil {
						return err
					}

					client := &http.Client{}
					resp, err := client.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusInternalServerError {
						return errors.New("bad status")
					}

					finalVal = "success"
					return nil
				})
				return finalVal, err
			}, retry.WithMaxAttempts(3)))

			_ = res.UnwrapOr("fallback")
		}(i)
	}

	// Test async Group under chaos
	g := async.NewGroup()
	for i := 0; i < 50; i++ {
		g.Go(func() error {
			<-startCh
			return nil
		})
	}

	close(startCh) // Unleash chaos
	wg.Wait()
	_ = g.Wait()
}
