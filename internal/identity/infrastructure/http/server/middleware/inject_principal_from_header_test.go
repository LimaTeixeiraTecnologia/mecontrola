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

type InjectPrincipalSuite struct {
	suite.Suite
}

func TestInjectPrincipalSuite(t *testing.T) {
	suite.Run(t, new(InjectPrincipalSuite))
}

func (s *InjectPrincipalSuite) TestInjectPrincipalFromHeader() {
	validUUID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	tests := []struct {
		name            string
		headerValue     string
		setHeader       bool
		expectPrincipal bool
		expectUUID      uuid.UUID
	}{
		{
			name:            "header ausente — segue sem principal",
			setHeader:       false,
			expectPrincipal: false,
		},
		{
			name:            "header malformado — segue sem principal",
			setHeader:       true,
			headerValue:     "not-a-uuid",
			expectPrincipal: false,
		},
		{
			name:            "nil UUID — segue sem principal",
			setHeader:       true,
			headerValue:     "00000000-0000-0000-0000-000000000000",
			expectPrincipal: false,
		},
		{
			name:            "UUID válido — injeta principal no ctx",
			setHeader:       true,
			headerValue:     validUUID.String(),
			expectPrincipal: true,
			expectUUID:      validUUID,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			var capturedReq *http.Request
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tc.setHeader {
				req.Header.Set("X-User-ID", tc.headerValue)
			}
			rr := httptest.NewRecorder()

			middleware.InjectPrincipalFromHeader(next).ServeHTTP(rr, req)

			s.False(rr.Code == http.StatusUnauthorized, "middleware must not write 401")
			s.Equal(http.StatusOK, rr.Code)

			if tc.expectPrincipal {
				s.Require().NotNil(capturedReq)
				p, ok := auth.FromContext(capturedReq.Context())
				s.True(ok)
				s.Equal(tc.expectUUID, p.UserID)
				s.Equal(auth.SourceHeader, p.Source)
			} else {
				if capturedReq != nil {
					_, ok := auth.FromContext(capturedReq.Context())
					s.False(ok)
				}
			}
		})
	}
}

func (s *InjectPrincipalSuite) TestChain_InjectThenRequire() {
	validUUID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := middleware.InjectPrincipalFromHeader(middleware.RequireUser(next))

	s.Run("header ausente resulta em 401 via RequireUser", func() {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rr := httptest.NewRecorder()
		chain.ServeHTTP(rr, req)
		s.Equal(http.StatusUnauthorized, rr.Code)
	})

	s.Run("header válido resulta em 200", func() {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("X-User-ID", validUUID.String())
		rr := httptest.NewRecorder()
		chain.ServeHTTP(rr, req)
		s.Equal(http.StatusOK, rr.Code)
	})
}
