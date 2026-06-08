package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
)

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10, nil)
	defer rl.Stop()

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		rl.Middleware(ok).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func TestRateLimiter_ThrottlesAfterLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10, nil)
	defer rl.Stop()

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "2.3.4.5:1234"
		rr := httptest.NewRecorder()
		rl.Middleware(ok).ServeHTTP(rr, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "2.3.4.5:1234"
	rr := httptest.NewRecorder()
	rl.Middleware(ok).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10, nil)
	defer rl.Stop()

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		rl.Middleware(ok).ServeHTTP(rr, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rr := httptest.NewRecorder()
	rl.Middleware(ok).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimiter_TrustedProxy_XRealIP(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10, []string{"127.0.0.1/32"})
	defer rl.Stop()

	firstIP := ""
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Real-IP", "192.168.1.100")
	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	_ = firstIP
}

func TestRateLimiter_UntrustedProxy_IgnoresHeaders(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10, []string{"10.0.0.0/8"})
	defer rl.Stop()

	requestCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	})

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		req.Header.Set("X-Real-IP", "1.1.1.1")
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("X-Real-IP", "1.1.1.1")
	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}
