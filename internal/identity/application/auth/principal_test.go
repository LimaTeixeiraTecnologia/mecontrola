package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type PrincipalSuite struct {
	suite.Suite
}

func TestPrincipalSuite(t *testing.T) {
	suite.Run(t, new(PrincipalSuite))
}

func (s *PrincipalSuite) TestRoundTrip() {
	tests := []struct {
		name      string
		principal auth.Principal
	}{
		{
			name: "whatsapp principal round-trip",
			principal: auth.Principal{
				UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Source: auth.SourceWhatsApp,
			},
		},
		{
			name: "different uuid",
			principal: auth.Principal{
				UserID: uuid.New(),
				Source: auth.SourceWhatsApp,
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx := auth.WithPrincipal(context.Background(), tc.principal)
			got, ok := auth.FromContext(ctx)
			s.True(ok)
			s.Equal(tc.principal, got)
		})
	}
}

func (s *PrincipalSuite) TestFromContext_EmptyCtx() {
	got, ok := auth.FromContext(context.Background())
	s.False(ok)
	s.True(got.IsZero())
}

func (s *PrincipalSuite) TestFromContext_ZeroPrincipal() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{})
	got, ok := auth.FromContext(ctx)
	s.False(ok)
	s.True(got.IsZero())
}

func (s *PrincipalSuite) TestIsZero() {
	tests := []struct {
		name     string
		p        auth.Principal
		expected bool
	}{
		{
			name:     "zero value",
			p:        auth.Principal{},
			expected: true,
		},
		{
			name:     "nil uuid",
			p:        auth.Principal{UserID: uuid.Nil, Source: auth.SourceWhatsApp},
			expected: true,
		},
		{
			name:     "non-zero",
			p:        auth.Principal{UserID: uuid.New(), Source: auth.SourceWhatsApp},
			expected: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			assert.Equal(s.T(), tc.expected, tc.p.IsZero())
		})
	}
}

func (s *PrincipalSuite) TestSourceWhatsApp_Value() {
	s.Equal(auth.PrincipalSource("whatsapp"), auth.SourceWhatsApp)
}

func (s *PrincipalSuite) TestSourceHeader_Value() {
	s.Equal(auth.PrincipalSource("header"), auth.SourceHeader)
}

func (s *PrincipalSuite) TestRoundTrip_SourceHeader() {
	p := auth.Principal{
		UserID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		Source: auth.SourceHeader,
	}
	ctx := auth.WithPrincipal(context.Background(), p)
	got, ok := auth.FromContext(ctx)
	s.True(ok)
	s.Equal(p, got)
	s.Equal(auth.SourceHeader, got.Source)
}

func BenchmarkWithPrincipal(b *testing.B) {
	p := auth.Principal{
		UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Source: auth.SourceWhatsApp,
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = auth.WithPrincipal(ctx, p)
	}
}

func BenchmarkFromContext(b *testing.B) {
	p := auth.Principal{
		UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Source: auth.SourceWhatsApp,
	}
	ctx := auth.WithPrincipal(context.Background(), p)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = auth.FromContext(ctx)
	}
}
