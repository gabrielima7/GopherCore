package simulation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"go.uber.org/goleak"
)

func TestChaos_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	router := httpkit.NewRouter()
	router.Get("/process", func(w http.ResponseWriter, r *http.Request) {
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "success"})
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numRequests = 500
	requests := make([]int, numRequests)
	for i := range requests {
		requests[i] = i
	}

	_, _ = async.Map(ctx, requests, 50, func(ctx context.Context, _ int) (bool, error) {
		err := cb.Execute(func() error {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/process", nil)
			resp, err := srv.Client().Do(req)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()
			return nil
		})
		return err == nil, err
	})
}
