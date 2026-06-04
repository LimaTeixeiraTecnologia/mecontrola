package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type WhatsAppNumberSuite struct {
	suite.Suite
}

func TestWhatsAppNumber(t *testing.T) {
	suite.Run(t, new(WhatsAppNumberSuite))
}

func (s *WhatsAppNumberSuite) TestValidInputs() {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		// 10 dígitos: DDD + 8 dígitos → injeta 55 e 9
		{name: "10 digitos DDD 11", input: "1188887777", expected: "+5511988887777"},
		{name: "10 digitos DDD 21", input: "2188887777", expected: "+5521988887777"},
		{name: "10 digitos DDD 31", input: "3188887777", expected: "+5531988887777"},
		{name: "10 digitos DDD 51", input: "5188887777", expected: "+5551988887777"},

		// 11 dígitos: DDD + 9 + 8 dígitos → injeta 55
		{name: "11 digitos DDD 11 com 9", input: "11988887777", expected: "+5511988887777"},
		{name: "11 digitos DDD 21 com 9", input: "21988887777", expected: "+5521988887777"},

		// 12 dígitos: 55 + DDD + 8 → injeta 9
		{name: "12 digitos com 55 DDD 11", input: "551188887777", expected: "+5511988887777"},
		{name: "12 digitos com 55 DDD 21", input: "552188887777", expected: "+5521988887777"},

		// 13 dígitos: já canônico
		{name: "13 digitos canonico DDD 11", input: "5511988887777", expected: "+5511988887777"},
		{name: "13 digitos canonico DDD 51", input: "5551988887777", expected: "+5551988887777"},

		// Com +55 (strips +)
		{name: "com +55 e 11 digitos", input: "+5511988887777", expected: "+5511988887777"},

		// Formato humano
		{name: "formato humano (11) 98888-7777", input: "(11) 98888-7777", expected: "+5511988887777"},
		{name: "formato humano com tracos", input: "11-98888-7777", expected: "+5511988887777"},

		// Idempotência
		{name: "idempotencia", input: "+5511988887777", expected: "+5511988887777"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewWhatsAppNumber(tc.input)
			s.NoError(err)
			s.Equal(tc.expected, got.String())
		})
	}
}

func (s *WhatsAppNumberSuite) TestInvalidInputs() {
	cases := []struct {
		name        string
		input       string
		expectedErr error
	}{
		{name: "vazio", input: "", expectedErr: valueobjects.ErrEmptyWhatsAppNumber},
		{name: "so espacos", input: "   ", expectedErr: valueobjects.ErrEmptyWhatsAppNumber},
		{name: "9 digitos", input: "119888877", expectedErr: valueobjects.ErrInvalidWhatsAppFormat},
		{name: "14 digitos", input: "55119888877770", expectedErr: valueobjects.ErrInvalidWhatsAppFormat},
		{name: "nao BR 12 digitos", input: "441188887777", expectedErr: valueobjects.ErrUnsupportedCountry},
		{name: "nao BR 13 digitos", input: "4411988887777", expectedErr: valueobjects.ErrUnsupportedCountry},
		{name: "so letras", input: "abcdefghijk", expectedErr: valueobjects.ErrEmptyWhatsAppNumber},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := valueobjects.NewWhatsAppNumber(tc.input)
			s.Error(err)
			s.True(errors.Is(err, tc.expectedErr), "esperado %v, got %v", tc.expectedErr, err)
		})
	}
}

func (s *WhatsAppNumberSuite) TestIdempotency() {
	input := "(11) 98888-7777"
	first, err := valueobjects.NewWhatsAppNumber(input)
	s.NoError(err)

	second, err := valueobjects.NewWhatsAppNumber(first.String())
	s.NoError(err)

	s.True(first.Equals(second))
	s.Equal(first.String(), second.String())
}

func (s *WhatsAppNumberSuite) TestIsZero() {
	var n valueobjects.WhatsAppNumber
	s.True(n.IsZero())

	n2, err := valueobjects.NewWhatsAppNumber("11988887777")
	s.NoError(err)
	s.False(n2.IsZero())
}

func (s *WhatsAppNumberSuite) TestEquals() {
	a, _ := valueobjects.NewWhatsAppNumber("11988887777")
	b, _ := valueobjects.NewWhatsAppNumber("+5511988887777")
	c, _ := valueobjects.NewWhatsAppNumber("21988887777")

	s.True(a.Equals(b))
	s.False(a.Equals(c))
}
