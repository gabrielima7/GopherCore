package simulation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
	// Initialize dbkit with SQLite
	dbPath := filepath.Join(t.TempDir(), "chaos_test.db")
	db := dbkit.MustConnect(context.Background(), "sqlite3", dbPath)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	}()
	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (name) VALUES (?)", "alice"); err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Initialize Server Router
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(10000, 20000),
		httpkit.WithCORS("*"),
	)

	router.Post("/process", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err != nil {
			httpkit.Error(w, http.StatusBadRequest, "failed to read body")
			return
		}

		var p Payload
		if err := jsonutil.Unmarshal(body, &p); err != nil {
			httpkit.Error(w, http.StatusBadRequest, "invalid payload")
			return
		}

		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok", "id": p.ID})
	})

	router.Get("/db", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM users WHERE name = ?", id)
		if err != nil {
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]int{"count": count})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	tests := []struct {
		name             string
		cbConfig         circuitbreaker.Config
		concurrencyLimit int
		payloadGenerator func(id int) []byte
		cancelRate       int // 1 in N requests get cancelled
	}{
		{
			name:             "Happy Path / Normal Traffic",
			cbConfig:         circuitbreaker.DefaultConfig(),
			concurrencyLimit: 100,
			payloadGenerator: func(id int) []byte { return []byte(`{"id": "normal"}`) },
			cancelRate:       0, // No cancellations
		},
		{
			name:             "Nil Configuration Handling (Circuit Breaker defaults)",
			cbConfig:         circuitbreaker.Config{}, // Zero-value config, testing defensive sanitization
			concurrencyLimit: 200,
			payloadGenerator: func(id int) []byte { return []byte(`{"id": "nil_config"}`) },
			cancelRate:       10, // 10% cancellations
		},
		{
			name:             "Database Connection Failure Simulation / SQL Injection Attempts",
			cbConfig:         circuitbreaker.DefaultConfig(),
			concurrencyLimit: 100,
			payloadGenerator: func(id int) []byte { return nil }, // Will test /db endpoint mostly
			cancelRate:       5,
		},
		{
			name:             "Malformed Payload / Unmarshalable Data",
			cbConfig:         circuitbreaker.DefaultConfig(),
			concurrencyLimit: 200,
			payloadGenerator: func(id int) []byte {
				if id%2 == 0 {
					return []byte(`{"id": 12345}`) // Invalid type for ID (expected string)
				}
				return []byte(`{"id": "malicious", "nested": {"nested": {"data": [1,2,3]}}}`)
			},
			cancelRate: 20,
		},
		{
			name:             "Extreme Concurrency / Rate Limit Saturation",
			cbConfig:         circuitbreaker.DefaultConfig(),
			concurrencyLimit: 2000,
			payloadGenerator: func(id int) []byte { return []byte(`{"id": "heavy_load"}`) },
			cancelRate:       8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := circuitbreaker.New(tt.cbConfig)
			var wg sync.WaitGroup
			startCh := make(chan struct{})

			for i := 0; i < tt.concurrencyLimit; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					<-startCh

					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()

					if tt.cancelRate > 0 && id%tt.cancelRate == 0 {
						cancel()
					}

					var isPost bool
					var endpoint string
					var payload []byte

					if tt.name == "Database Connection Failure Simulation / SQL Injection Attempts" || id%2 != 0 {
						isPost = false
						endpoint = srv.URL + "/db?id=';+DROP+TABLE+users;+--"
					} else {
						isPost = true
						endpoint = srv.URL + "/process"
						payload = tt.payloadGenerator(id)
					}

					res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
						var finalVal string
						err := cb.Execute(func() (err error) {
							var req *http.Request
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
							defer func() {
								closeErr := resp.Body.Close()
								if closeErr != nil && err == nil {
									err = closeErr
								}
							}()

							if resp.StatusCode >= http.StatusBadRequest {
								return errors.New("bad status")
							}

							finalVal = "success"
							return nil
						})
						return finalVal, err
					}, retry.WithMaxAttempts(2)))

					err := res.Error()
					if err != nil {
						if !errors.Is(err, circuitbreaker.ErrCircuitOpen) &&
							!errors.Is(err, circuitbreaker.ErrTooManyRequests) &&
							!errors.Is(err, context.DeadlineExceeded) &&
							!errors.Is(err, context.Canceled) &&
							err.Error() != "bad status" &&
							!strings.Contains(err.Error(), "max attempts reached") &&
							!strings.Contains(err.Error(), "deadline exceeded") &&
							!strings.Contains(err.Error(), "context canceled") {
							t.Errorf("unexpected error in chaos testing: %v", err)
						}
					}
				}(i)
			}

			g := async.NewGroup()
			for i := 0; i < 20; i++ {
				g.Go(func() error {
					<-startCh
					return nil
				})
			}

			close(startCh)
			wg.Wait()
			_ = g.Wait()
		})
	}
}
