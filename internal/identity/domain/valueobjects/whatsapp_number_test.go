package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type WhatsappNumberSuite struct {
	suite.Suite
}

func TestWhatsappNumberSuite(t *testing.T) {
	suite.Run(t, new(WhatsappNumberSuite))
}

func (s *WhatsappNumberSuite) SetupTest() {}

func (s *WhatsappNumberSuite) mustWhatsApp(raw string) valueobjects.WhatsAppNumber {
	whatsApp, err := valueobjects.NewWhatsAppNumber(raw)
	s.Require().NoError(err)
	return whatsApp
}

func (s *WhatsappNumberSuite) TestNewWhatsAppNumber() {
	type args struct {
		input string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(valueobjects.WhatsAppNumber, error)
	}{
		{
			name: "deve normalizar numero canonico",
			args: args{input: "(11) 98888-7777"},
			expect: func(number valueobjects.WhatsAppNumber, err error) {
				s.Require().NoError(err)
				s.Equal("+5511988887777", number.String())
			},
		},
		{
			name: "deve aceitar numero com codigo do pais sem sinal",
			args: args{input: "5511988887777"},
			expect: func(number valueobjects.WhatsAppNumber, err error) {
				s.Require().NoError(err)
				s.Equal("+5511988887777", number.String())
			},
		},
		{
			name: "deve retornar erro para valor vazio",
			args: args{input: ""},
			expect: func(number valueobjects.WhatsAppNumber, err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, valueobjects.ErrWhatsAppNumberEmpty)
				s.Equal(valueobjects.WhatsAppNumber{}, number)
			},
		},
		{
			name: "deve retornar erro para numero invalido",
			args: args{input: "+551133334444"},
			expect: func(number valueobjects.WhatsAppNumber, err error) {
				s.Require().Error(err)
				s.True(errors.Is(err, valueobjects.ErrWhatsAppNumberInvalid))
				s.Equal(valueobjects.WhatsAppNumber{}, number)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			number, err := valueobjects.NewWhatsAppNumber(scenario.args.input)
			scenario.expect(number, err)
		})
	}
}

func (s *WhatsappNumberSuite) TestEqual() {
	type args struct {
		left  string
		right string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(bool)
	}{
		{
			name: "deve considerar numeros equivalentes",
			args: args{left: "+5511988887777", right: "(11) 98888-7777"},
			expect: func(equal bool) {
				s.True(equal)
			},
		},
		{
			name: "deve diferenciar numeros distintos",
			args: args{left: "+5511988887777", right: "+5521987654321"},
			expect: func(equal bool) {
				s.False(equal)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			left := s.mustWhatsApp(scenario.args.left)
			right := s.mustWhatsApp(scenario.args.right)
			scenario.expect(left.Equal(right))
		})
	}
}

func (s *WhatsappNumberSuite) TestMasked() {
	type args struct {
		input   string
		useZero bool
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(string)
	}{
		{
			name: "deve mascarar numero preenchido",
			args: args{input: "+5511988887777"},
			expect: func(masked string) {
				s.Equal("+55 11 9****-7777", masked)
			},
		},
		{
			name: "deve mascarar zero value",
			args: args{useZero: true},
			expect: func(masked string) {
				s.Equal("****", masked)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var number valueobjects.WhatsAppNumber
			if !scenario.args.useZero {
				number = s.mustWhatsApp(scenario.args.input)
			}

			scenario.expect(number.Masked())
		})
	}
}
