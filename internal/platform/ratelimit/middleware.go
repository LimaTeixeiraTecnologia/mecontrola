package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
)

type RateLimitConfig struct {
	PerMinute int
	Burst     int
	Extractor KeyExtractor
	Scope     string
}

type keyLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimitMiddleware struct {
	mu       sync.Mutex
	limiters map[string]*keyLimiter
	rps      rate.Limit
	burst    int
	extract  KeyExtractor
	scope    string
	counter  observability.Counter
}

func NewRateLimitMiddleware(cfg RateLimitConfig, o11y observability.Observability) func(http.Handler) http.Handler {
	m := &rateLimitMiddleware{
		limiters: make(map[string]*keyLimiter),
		rps:      rate.Limit(float64(cfg.PerMinute) / 60.0),
		burst:    cfg.Burst,
		extract:  cfg.Extractor,
		scope:    cfg.Scope,
		counter:  o11y.Metrics().Counter("auth_rate_limit_exceeded_total", "Total auth rate limit rejections by scope", "1"),
	}
	go m.gcLoop()
	return m.handle
}

func (m *rateLimitMiddleware) handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := m.extract(r)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !m.limiterFor(key).Allow() {
			m.counter.Increment(r.Context(), observability.String("scope", m.scope))
			responses.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *rateLimitMiddleware) limiterFor(key string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.limiters[key]
	if !ok {
		l = &keyLimiter{
			limiter:  rate.NewLimiter(m.rps, m.burst),
			lastSeen: time.Now().UTC(),
		}
		m.limiters[key] = l
	}
	l.lastSeen = time.Now().UTC()
	return l.limiter
}

func (m *rateLimitMiddleware) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.gc()
	}
}

func (m *rateLimitMiddleware) gc() {
	cutoff := time.Now().UTC().Add(-10 * time.Minute)
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, l := range m.limiters {
		if l.lastSeen.Before(cutoff) {
			delete(m.limiters, k)
		}
	}
}
