package valueobjects_test

import (
	"github.com/stretchr/testify/suite"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type FunnelTokenSuite struct {
	suite.Suite
}

func TestFunnelTokenSuite(t *testing.T) {
	suite.Run(t, new(FunnelTokenSuite))
}

func (s *FunnelTokenSuite) SetupTest() {}

func (s *FunnelTokenSuite) TestNewFunnelToken() {
	type args struct {
		raw string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(valueobjects.FunnelToken, error)
	}{
		{
			name: "deve aceitar token valido",
			args: args{raw: "token-123"},
			expect: func(token valueobjects.FunnelToken, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), "token-123", token.String())
			},
		},
		{
			name: "deve remover espacos nas extremidades",
			args: args{raw: "  token-123  "},
			expect: func(token valueobjects.FunnelToken, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), "token-123", token.String())
			},
		},
		{
			name: "deve rejeitar token vazio",
			args: args{raw: ""},
			expect: func(token valueobjects.FunnelToken, err error) {
				require.ErrorIs(s.T(), err, valueobjects.ErrFunnelTokenEmpty)
				assert.Empty(s.T(), token.String())
			},
		},
		{
			name: "deve rejeitar token em branco",
			args: args{raw: "   "},
			expect: func(token valueobjects.FunnelToken, err error) {
				require.ErrorIs(s.T(), err, valueobjects.ErrFunnelTokenEmpty)
				assert.Empty(s.T(), token.String())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			token, err := valueobjects.NewFunnelToken(scenario.args.raw)
			scenario.expect(token, err)
		})
	}
}
