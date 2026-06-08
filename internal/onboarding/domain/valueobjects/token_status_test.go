package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TokenStatusSuite struct {
	suite.Suite
}

func TestTokenStatusSuite(t *testing.T) {
	suite.Run(t, new(TokenStatusSuite))
}

func (s *TokenStatusSuite) SetupTest() {}

func (s *TokenStatusSuite) TestParseTokenStatus() {
	type args struct {
		raw string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(status valueobjects.TokenStatus, err error)
	}{
		{
			name: "deve converter status validos",
			args: args{raw: "PENDING"},
			expect: func(status valueobjects.TokenStatus, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusPending, status)
				s.Equal("PENDING", status.String())
			},
		},
		{
			name: "deve converter status pagos",
			args: args{raw: "PAID"},
			expect: func(status valueobjects.TokenStatus, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusPaid, status)
				s.Equal("PAID", status.String())
			},
		},
		{
			name: "deve converter status consumidos",
			args: args{raw: "CONSUMED"},
			expect: func(status valueobjects.TokenStatus, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusConsumed, status)
				s.Equal("CONSUMED", status.String())
			},
		},
		{
			name: "deve converter status expirados",
			args: args{raw: "EXPIRED"},
			expect: func(status valueobjects.TokenStatus, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusExpired, status)
				s.Equal("EXPIRED", status.String())
			},
		},
		{
			name: "deve retornar erro para status invalido",
			args: args{raw: "INVALID"},
			expect: func(status valueobjects.TokenStatus, err error) {
				s.ErrorIs(err, valueobjects.ErrTokenStatusInvalid)
				s.Zero(status)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			status, err := valueobjects.ParseTokenStatus(scenario.args.raw)
			scenario.expect(status, err)
		})
	}
}

func (s *TokenStatusSuite) TestString() {
	type args struct {
		status valueobjects.TokenStatus
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(string)
	}{
		{
			name: "deve retornar unknown para zero value",
			args: args{},
			expect: func(got string) {
				s.Equal("UNKNOWN", got)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(scenario.args.status.String())
		})
	}
}
