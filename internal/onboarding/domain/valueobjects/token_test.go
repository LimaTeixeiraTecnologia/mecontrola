package valueobjects_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TokenSuite struct {
	suite.Suite
}

func TestTokenSuite(t *testing.T) {
	suite.Run(t, new(TokenSuite))
}

func (s *TokenSuite) SetupTest() {}

func (s *TokenSuite) TestNewToken() {
	scenarios := []struct {
		name   string
		expect func()
	}{
		{
			name: "deve gerar tokens unicos",
			expect: func() {
				firstToken, err := valueobjects.NewToken()
				s.Require().NoError(err)

				secondToken, err := valueobjects.NewToken()
				s.Require().NoError(err)

				s.NotEqual(firstToken.ClearText(), secondToken.ClearText())
			},
		},
		{
			name: "deve gerar clear text com 43 caracteres",
			expect: func() {
				token, err := valueobjects.NewToken()
				s.Require().NoError(err)
				s.Len(token.ClearText(), 43)
			},
		},
		{
			name: "deve gerar clear text url safe",
			expect: func() {
				token, err := valueobjects.NewToken()
				s.Require().NoError(err)
				clearText := token.ClearText()
				s.False(strings.Contains(clearText, "+"))
				s.False(strings.Contains(clearText, "/"))
				s.False(strings.Contains(clearText, "="))
			},
		},
		{
			name: "deve gerar hash com 32 bytes",
			expect: func() {
				token, err := valueobjects.NewToken()
				s.Require().NoError(err)
				s.Len(token.Hash(), 32)
			},
		},
		{
			name: "deve gerar hash deterministico",
			expect: func() {
				token, err := valueobjects.NewToken()
				s.Require().NoError(err)

				recoveredToken, err := valueobjects.TokenFromClear(token.ClearText())
				s.Require().NoError(err)
				s.Equal(token.HashHex(), recoveredToken.HashHex())
			},
		},
		{
			name: "deve mascarar string do token",
			expect: func() {
				token, err := valueobjects.NewToken()
				s.Require().NoError(err)
				s.Equal("[REDACTED]", token.String())
				s.NotContains(token.String(), token.ClearText())
			},
		},
		{
			name: "deve retornar hash prefix com 8 caracteres",
			expect: func() {
				token, err := valueobjects.NewToken()
				s.Require().NoError(err)
				s.Len(token.HashPrefix(), 8)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, scenario.expect)
	}
}

func (s *TokenSuite) TestTokenFromClear() {
	type args struct {
		clearText string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(valueobjects.Token, error)
	}{
		{
			name: "deve retornar erro para token vazio",
			args: args{clearText: ""},
			expect: func(token valueobjects.Token, err error) {
				s.ErrorIs(err, valueobjects.ErrTokenEmpty)
				s.Zero(token)
			},
		},
		{
			name: "deve retornar erro para base64 invalido",
			args: args{clearText: "not-valid-base64!!"},
			expect: func(token valueobjects.Token, err error) {
				s.ErrorIs(err, valueobjects.ErrTokenInvalid)
				s.Zero(token)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			token, err := valueobjects.TokenFromClear(scenario.args.clearText)
			scenario.expect(token, err)
		})
	}
}
