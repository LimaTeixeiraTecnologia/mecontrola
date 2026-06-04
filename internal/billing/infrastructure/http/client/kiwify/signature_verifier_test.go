package kiwify_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

type TokenSignatureVerifierSuite struct {
	suite.Suite
}

func TestTokenSignatureVerifier(t *testing.T) {
	suite.Run(t, new(TokenSignatureVerifierSuite))
}

func (s *TokenSignatureVerifierSuite) TestVerify() {
	const expectedToken = "super-secret-token"
	verifier := kiwify.NewTokenSignatureVerifier(expectedToken, "X-Kiwify-Webhook-Token")

	type testCase struct {
		name        string
		headers     map[string]string
		wantError   error
		expectNoErr bool
	}

	cases := []testCase{
		{
			name:        "header presente com valor correto",
			headers:     map[string]string{"X-Kiwify-Webhook-Token": expectedToken},
			expectNoErr: true,
		},
		{
			name:      "header ausente",
			headers:   map[string]string{},
			wantError: kiwify.ErrMissingSignature,
		},
		{
			name:      "header presente com valor errado",
			headers:   map[string]string{"X-Kiwify-Webhook-Token": "wrong-token"},
			wantError: kiwify.ErrInvalidSignature,
		},
		{
			name:        "header em lowercase é aceito (case-insensitive)",
			headers:     map[string]string{"x-kiwify-webhook-token": expectedToken},
			expectNoErr: true,
		},
		{
			name:        "header em uppercase é aceito (case-insensitive)",
			headers:     map[string]string{"X-KIWIFY-WEBHOOK-TOKEN": expectedToken},
			expectNoErr: true,
		},
		{
			name:        "header em mixed case é aceito (case-insensitive)",
			headers:     map[string]string{"x-Kiwify-Webhook-Token": expectedToken},
			expectNoErr: true,
		},
		{
			name:      "header com valor vazio",
			headers:   map[string]string{"X-Kiwify-Webhook-Token": ""},
			wantError: kiwify.ErrMissingSignature,
		},
		{
			name:      "múltiplos headers sem o esperado",
			headers:   map[string]string{"Content-Type": "application/json", "Authorization": "Bearer abc"},
			wantError: kiwify.ErrMissingSignature,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			err := verifier.Verify([]byte("payload"), tc.headers)
			if tc.expectNoErr {
				s.NoError(err)
				return
			}
			s.ErrorIs(err, tc.wantError)
		})
	}
}

func (s *TokenSignatureVerifierSuite) TestVerify_CustomHeaderName() {
	verifier := kiwify.NewTokenSignatureVerifier("my-token", "X-Custom-Signature")
	err := verifier.Verify(nil, map[string]string{"x-custom-signature": "my-token"})
	s.NoError(err)
}
