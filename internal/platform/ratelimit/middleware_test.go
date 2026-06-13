package ratelimit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/ratelimit"
)

type MiddlewareSuite struct {
	suite.Suite
}

func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareSuite))
}

func (s *MiddlewareSuite) newMiddleware(extractor ratelimit.KeyExtractor, scope string, perMin, burst int) func(http.Handler) http.Handler {
	return ratelimit.NewRateLimitMiddleware(ratelimit.RateLimitConfig{
		PerMinute: perMin,
		Burst:     burst,
		Extractor: extractor,
		Scope:     scope,
	}, noop.NewProvider())
}

func (s *MiddlewareSuite) ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (s *MiddlewareSuite) TestByIPAllowsUpToLimit() {
	mw := s.newMiddleware(ratelimit.ByIP, "ip", 10, 3)
	handler := mw(s.ok())

	for range 3 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "1.2.3.4:9999"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		s.Equal(http.StatusOK, rec.Code)
	}
}

func (s *MiddlewareSuite) TestByIPBlocksAfterBurst() {
	mw := s.newMiddleware(ratelimit.ByIP, "ip", 10, 2)
	handler := mw(s.ok())

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "9.9.9.9:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "9.9.9.9:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	s.Equal(http.StatusTooManyRequests, rec.Code)
}

func (s *MiddlewareSuite) TestByUserIDBlocksAfterBurst() {
	mw := s.newMiddleware(ratelimit.ByUserID, "user", 10, 2)
	handler := mw(s.ok())

	userID := uuid.New()
	principalCtx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: userID, Source: auth.SourceHeader})

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(principalCtx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(principalCtx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	s.Equal(http.StatusTooManyRequests, rec.Code)
}

func (s *MiddlewareSuite) TestByUserIDPassthroughWhenNoPrincipal() {
	mw := s.newMiddleware(ratelimit.ByUserID, "user", 10, 1)
	handler := mw(s.ok())

	for range 5 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		s.Equal(http.StatusOK, rec.Code)
	}
}

func (s *MiddlewareSuite) TestByUserIDFallbackIPUsesIPWhenNoPrincipal() {
	mw := s.newMiddleware(ratelimit.ByUserIDFallbackIP, "user", 10, 2)
	handler := mw(s.ok())

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "4.4.4.4:80"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "4.4.4.4:80"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	s.Equal(http.StatusTooManyRequests, rec.Code)
}

func (s *MiddlewareSuite) TestByUserIDFallbackIPUsesUserIDWhenPrincipalPresent() {
	mw := s.newMiddleware(ratelimit.ByUserIDFallbackIP, "user", 10, 2)
	handler := mw(s.ok())

	userID := uuid.New()
	principalCtx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: userID, Source: auth.SourceHeader})

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(principalCtx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(principalCtx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	s.Equal(http.StatusTooManyRequests, rec.Code)

	otherUserID := uuid.New()
	otherCtx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: otherUserID, Source: auth.SourceHeader})
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2 = r2.WithContext(otherCtx)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, r2)
	s.Equal(http.StatusOK, rec2.Code)
}

func (s *MiddlewareSuite) TestDifferentUsersAreIndependent() {
	mw := s.newMiddleware(ratelimit.ByUserID, "user", 10, 2)
	handler := mw(s.ok())

	user1 := uuid.New()
	user2 := uuid.New()
	ctx1 := auth.WithPrincipal(context.Background(), auth.Principal{UserID: user1, Source: auth.SourceHeader})
	ctx2 := auth.WithPrincipal(context.Background(), auth.Principal{UserID: user2, Source: auth.SourceHeader})

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(ctx1)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
	}

	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1 = r1.WithContext(ctx1)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, r1)
	s.Equal(http.StatusTooManyRequests, rec1.Code)

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2 = r2.WithContext(ctx2)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, r2)
	s.Equal(http.StatusOK, rec2.Code)
}
