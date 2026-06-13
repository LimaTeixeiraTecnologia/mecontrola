package ratelimit_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/ratelimit"
)

type ExtractorSuite struct {
	suite.Suite
}

func TestExtractorSuite(t *testing.T) {
	suite.Run(t, new(ExtractorSuite))
}

func (s *ExtractorSuite) TestByIP() {
	scenarios := []struct {
		name       string
		remoteAddr string
		xRealIP    string
		want       string
	}{
		{
			name:       "retorna IP do RemoteAddr sem header",
			remoteAddr: "1.2.3.4:5678",
			want:       "1.2.3.4",
		},
		{
			name:       "retorna X-Real-IP quando presente",
			remoteAddr: "127.0.0.1:1234",
			xRealIP:    "203.0.113.10",
			want:       "203.0.113.10",
		},
		{
			name:       "retorna RemoteAddr quando nao ha porta",
			remoteAddr: "10.0.0.5",
			want:       "10.0.0.5",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = sc.remoteAddr
			if sc.xRealIP != "" {
				r.Header.Set("X-Real-IP", sc.xRealIP)
			}
			got := ratelimit.ByIP(r)
			s.Equal(sc.want, got)
		})
	}
}

func (s *ExtractorSuite) TestByUserID() {
	userID := uuid.New()

	scenarios := []struct {
		name    string
		withCtx bool
		want    string
	}{
		{
			name:    "retorna UserID quando principal esta no context",
			withCtx: true,
			want:    userID.String(),
		},
		{
			name:    "retorna vazio quando nao ha principal",
			withCtx: false,
			want:    "",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			r := httptest.NewRequest("GET", "/", nil)
			if sc.withCtx {
				ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: userID, Source: auth.SourceHeader})
				r = r.WithContext(ctx)
			}
			got := ratelimit.ByUserID(r)
			s.Equal(sc.want, got)
		})
	}
}

func (s *ExtractorSuite) TestByUserIDFallbackIP() {
	userID := uuid.New()

	scenarios := []struct {
		name       string
		withCtx    bool
		remoteAddr string
		want       string
	}{
		{
			name:       "retorna UserID quando principal esta no context",
			withCtx:    true,
			remoteAddr: "1.2.3.4:1234",
			want:       userID.String(),
		},
		{
			name:       "retorna IP quando nao ha principal",
			withCtx:    false,
			remoteAddr: "5.6.7.8:1234",
			want:       "5.6.7.8",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = sc.remoteAddr
			if sc.withCtx {
				ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: userID, Source: auth.SourceHeader})
				r = r.WithContext(ctx)
			}
			got := ratelimit.ByUserIDFallbackIP(r)
			s.Equal(sc.want, got)
		})
	}
}
