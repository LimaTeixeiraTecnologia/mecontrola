package phone

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MobileSuite struct {
	suite.Suite
}

func TestMobileSuite(t *testing.T) {
	suite.Run(t, new(MobileSuite))
}

func (s *MobileSuite) SetupTest() {}

func (s *MobileSuite) TestNormalizeBR() {
	type args struct {
		raw string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(string, error)
	}{
		{
			name: "deve normalizar numero local sem prefixo",
			args: args{raw: "11999999999"},
			expect: func(e164 string, err error) {
				s.Require().NoError(err)
				s.Equal("+5511999999999", e164)
			},
		},
		{
			name: "deve normalizar numero com prefixo 55 sem sinal",
			args: args{raw: "5511999999999"},
			expect: func(e164 string, err error) {
				s.Require().NoError(err)
				s.Equal("+5511999999999", e164)
			},
		},
		{
			name: "deve aceitar numero ja em E.164",
			args: args{raw: "+5511999999999"},
			expect: func(e164 string, err error) {
				s.Require().NoError(err)
				s.Equal("+5511999999999", e164)
			},
		},
		{
			name: "deve normalizar numero com espacos, parenteses e hifen",
			args: args{raw: "(11) 99999-9999"},
			expect: func(e164 string, err error) {
				s.Require().NoError(err)
				s.Equal("+5511999999999", e164)
			},
		},
		{
			name: "deve retornar erro para numero vazio",
			args: args{raw: ""},
			expect: func(e164 string, err error) {
				s.Require().Error(err)
				s.ErrorIs(err, ErrMobileEmpty)
				s.Empty(e164)
			},
		},
		{
			name: "deve retornar erro para numero invalido (fixo, sem digito 9)",
			args: args{raw: "+551133334444"},
			expect: func(e164 string, err error) {
				s.Require().Error(err)
				s.True(errors.Is(err, ErrMobileInvalid))
				s.Empty(e164)
			},
		},
		{
			name: "deve retornar erro para numero invalido curto",
			args: args{raw: "999"},
			expect: func(e164 string, err error) {
				s.Require().Error(err)
				s.True(errors.Is(err, ErrMobileInvalid))
				s.Empty(e164)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			e164, err := NormalizeBR(scenario.args.raw)
			scenario.expect(e164, err)
		})
	}
}

func (s *MobileSuite) TestNewMobileBR() {
	type args struct {
		raw string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(Mobile, error)
	}{
		{
			name: "deve criar Mobile valido",
			args: args{raw: "+5511999999999"},
			expect: func(m Mobile, err error) {
				s.Require().NoError(err)
				s.Equal("+5511999999999", m.String())
			},
		},
		{
			name: "deve retornar erro para vazio",
			args: args{raw: ""},
			expect: func(m Mobile, err error) {
				s.Require().Error(err)
				s.ErrorIs(err, ErrMobileEmpty)
				s.Equal(Mobile{}, m)
			},
		},
		{
			name: "deve retornar erro para numero invalido",
			args: args{raw: "abc"},
			expect: func(m Mobile, err error) {
				s.Require().Error(err)
				s.True(errors.Is(err, ErrMobileInvalid))
				s.Equal(Mobile{}, m)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			m, err := NewMobileBR(scenario.args.raw)
			scenario.expect(m, err)
		})
	}
}
