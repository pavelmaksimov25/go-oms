package health

import (
	"context"
	"net/http"
	"time"
)

type Checker interface {
	Check(ctx context.Context) error
}

type CheckerFunc func(ctx context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error { return f(ctx) }

const readyTimeout = 2 * time.Second

// NewHandler returns an http.Handler serving /healthz and /readyz.
// Kept as a convenience for callers that don't need other routes.
func NewHandler(readyChecks ...Checker) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, readyChecks...)
	return mux
}

// RegisterRoutes adds /healthz and /readyz to the given mux so the caller
// can combine them with other routes like /metrics on a single HTTP server.
func RegisterRoutes(mux *http.ServeMux, readyChecks ...Checker) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		for _, c := range readyChecks {
			if err := c.Check(ctx); err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
}
