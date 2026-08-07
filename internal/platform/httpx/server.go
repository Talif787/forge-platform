package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type Server struct {
	http *http.Server
}

func NewServer(addr string, handler http.Handler) *Server {
	return &Server{
		http: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

type HealthChecker func(ctx context.Context) error

// Liveness reflects process health only; it must not check dependencies, so a
// dependency outage does not cause the orchestrator to restart the pod.
func Liveness() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Readiness verifies critical dependencies; failure removes the instance from
// load balancing until it recovers.
func Readiness(ready HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unready", "reason": err.Error()})
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
