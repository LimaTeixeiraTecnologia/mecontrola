package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
)

type RateLimitSuite struct {
	suite.Suite
}

func TestRateLimitSuite(t *testing.T) {
	suite.Run(t, new(RateLimitSuite))
}

func (s *RateLimitSuite) SetupTest() {}

func (s *RateLimitSuite) TestRateLimiter() {
	type args struct {
		remoteAddrs    []string
		trustedProxies []string
		setHeaders     func(*http.Request)
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(*middleware.RateLimiter, http.Handler)
	}{
		{
			name: "deve permitir ate o limite",
			args: args{
				remoteAddrs: []string{
					"1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234",
					"1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234",
				},
			},
			expect: func(limiter *middleware.RateLimiter, next http.Handler) {
				for _, remoteAddr := range []string{
					"1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234",
					"1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234", "1.2.3.4:1234",
				} {
					request := httptest.NewRequest(http.MethodGet, "/", nil)
					request.RemoteAddr = remoteAddr
					recorder := httptest.NewRecorder()
					limiter.Middleware(next).ServeHTTP(recorder, request)
					s.Equal(http.StatusOK, recorder.Code)
				}
			},
		},
		{
			name: "deve bloquear apos o limite",
			args: args{},
			expect: func(limiter *middleware.RateLimiter, next http.Handler) {
				for range 10 {
					request := httptest.NewRequest(http.MethodGet, "/", nil)
					request.RemoteAddr = "2.3.4.5:1234"
					recorder := httptest.NewRecorder()
					limiter.Middleware(next).ServeHTTP(recorder, request)
				}

				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.RemoteAddr = "2.3.4.5:1234"
				recorder := httptest.NewRecorder()
				limiter.Middleware(next).ServeHTTP(recorder, request)
				s.Equal(http.StatusTooManyRequests, recorder.Code)
			},
		},
		{
			name: "deve tratar ips diferentes de forma independente",
			args: args{},
			expect: func(limiter *middleware.RateLimiter, next http.Handler) {
				for range 10 {
					request := httptest.NewRequest(http.MethodGet, "/", nil)
					request.RemoteAddr = "10.0.0.1:1234"
					recorder := httptest.NewRecorder()
					limiter.Middleware(next).ServeHTTP(recorder, request)
				}

				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.RemoteAddr = "10.0.0.2:1234"
				recorder := httptest.NewRecorder()
				limiter.Middleware(next).ServeHTTP(recorder, request)
				s.Equal(http.StatusOK, recorder.Code)
			},
		},
		{
			name: "deve confiar em proxy autorizado com x-real-ip",
			args: args{trustedProxies: []string{"127.0.0.1/32"}},
			expect: func(limiter *middleware.RateLimiter, next http.Handler) {
				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.RemoteAddr = "127.0.0.1:1234"
				request.Header.Set("X-Real-IP", "192.168.1.100")
				recorder := httptest.NewRecorder()
				limiter.Middleware(next).ServeHTTP(recorder, request)
				s.Equal(http.StatusOK, recorder.Code)
			},
		},
		{
			name: "deve ignorar header em proxy nao confiavel",
			args: args{trustedProxies: []string{"10.0.0.0/8"}},
			expect: func(limiter *middleware.RateLimiter, next http.Handler) {
				for range 10 {
					request := httptest.NewRequest(http.MethodGet, "/", nil)
					request.RemoteAddr = "192.168.1.1:1234"
					request.Header.Set("X-Real-IP", "1.1.1.1")
					recorder := httptest.NewRecorder()
					limiter.Middleware(next).ServeHTTP(recorder, request)
				}

				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.RemoteAddr = "192.168.1.1:1234"
				request.Header.Set("X-Real-IP", "1.1.1.1")
				recorder := httptest.NewRecorder()
				limiter.Middleware(next).ServeHTTP(recorder, request)
				s.Equal(http.StatusTooManyRequests, recorder.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			limiter := middleware.NewRateLimiter(10, 10, scenario.args.trustedProxies)
			defer limiter.Stop()

			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			scenario.expect(limiter, next)
		})
	}
}
