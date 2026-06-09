package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
)

type RequireUserSuite struct {
	suite.Suite
}

func TestRequireUserSuite(t *testing.T) {
	suite.Run(t, new(RequireUserSuite))
}

func (s *RequireUserSuite) TestRequireUser() {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		setupCtx       func(r *http.Request) *http.Request
		expectedStatus int
		expectedBody   string
		expectedCT     string
		nextCalled     bool
	}{
		{
			name: "sem principal retorna 401",
			setupCtx: func(r *http.Request) *http.Request {
				return r
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"message":"unauthorized"}`,
			expectedCT:     "application/json",
			nextCalled:     false,
		},
		{
			name: "com principal chama next",
			setupCtx: func(r *http.Request) *http.Request {
				p := auth.Principal{
					UserID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
					Source: auth.SourceWhatsApp,
				}
				return r.WithContext(auth.WithPrincipal(r.Context(), p))
			},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req = tc.setupCtx(req)
			rr := httptest.NewRecorder()

			handler := middleware.RequireUser(next)
			handler.ServeHTTP(rr, req)

			s.Equal(tc.expectedStatus, rr.Code)
			s.Equal(tc.nextCalled, nextCalled)
			if tc.expectedBody != "" {
				s.Equal(tc.expectedBody, rr.Body.String())
			}
			if tc.expectedCT != "" {
				s.Equal(tc.expectedCT, rr.Header().Get("Content-Type"))
			}
		})
	}
}

// BenchmarkRequireUser_NoPrincipal alvo overhead < 1 µs/op.
func BenchmarkRequireUser_NoPrincipal(b *testing.B) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequireUser(next)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

// BenchmarkRequireUser_WithPrincipal alvo overhead < 1 µs/op.
func BenchmarkRequireUser_WithPrincipal(b *testing.B) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequireUser(next)
	p := auth.Principal{
		UserID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Source: auth.SourceWhatsApp,
	}
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(auth.WithPrincipal(req.Context(), p))

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
