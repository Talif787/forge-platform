package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/forge-platform/forge/internal/platform/apperr"
	"github.com/forge-platform/forge/internal/platform/log"
	"github.com/forge-platform/forge/internal/platform/observability"
)

const correlationHeader = "X-Correlation-Id"

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.From(r.Context()).Error("panic recovered", "panic", rec, "stack", string(debug.Stack()))
				Error(w, r, apperr.Internal("PANIC", "an unexpected error occurred"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Correlate assigns or propagates a correlation id and derives a request-scoped
// logger carrying the correlation id and, when present, the trace id.
func Correlate(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cid := r.Header.Get(correlationHeader)
			if cid == "" {
				cid = uuid.NewString()
			}
			ctx := context.WithValue(r.Context(), ctxCorrelationID, cid)
			reqLogger := base.With(slog.String("correlation_id", cid))
			if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
				reqLogger = reqLogger.With(slog.String("trace_id", sc.TraceID().String()))
			}
			w.Header().Set(correlationHeader, cid)
			next.ServeHTTP(w, r.WithContext(log.Into(ctx, reqLogger)))
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Observe(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			m.HTTPDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
			m.HTTPRequests.WithLabelValues(route, r.Method, http.StatusText(rec.status)).Inc()
			log.From(r.Context()).Info("request",
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
