package httpx

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/forge-platform/forge/internal/platform/apperr"
)

// keyedLimiter holds a token-bucket limiter per identity. It is in-process for
// Phase 1; a Redis-backed limiter replaces it for multi-instance enforcement.
type keyedLimiter struct {
	mu      sync.Mutex
	buckets map[string]*entry
	rps     rate.Limit
	burst   int
	ttl     time.Duration
}

type entry struct {
	limiter *rate.Limiter
	seen    time.Time
}

func newKeyedLimiter(rps float64, burst int) *keyedLimiter {
	kl := &keyedLimiter{
		buckets: map[string]*entry{},
		rps:     rate.Limit(rps),
		burst:   burst,
		ttl:     10 * time.Minute,
	}
	go kl.reap()
	return kl
}

func (kl *keyedLimiter) allow(key string) bool {
	kl.mu.Lock()
	defer kl.mu.Unlock()
	e, ok := kl.buckets[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(kl.rps, kl.burst)}
		kl.buckets[key] = e
	}
	e.seen = time.Now()
	return e.limiter.Allow()
}

func (kl *keyedLimiter) reap() {
	t := time.NewTicker(kl.ttl)
	defer t.Stop()
	for range t.C {
		kl.mu.Lock()
		for k, e := range kl.buckets {
			if time.Since(e.seen) > kl.ttl {
				delete(kl.buckets, k)
			}
		}
		kl.mu.Unlock()
	}
}

// RateLimit limits per authenticated subject, falling back to remote address
// for unauthenticated routes.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	kl := newKeyedLimiter(rps, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if p, ok := PrincipalFrom(r.Context()); ok && p.Subject != "" {
				key = p.Subject
			}
			if !kl.allow(key) {
				w.Header().Set("Retry-After", "1")
				Error(w, r, apperr.TooManyRequests("RATE_LIMITED", "rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
