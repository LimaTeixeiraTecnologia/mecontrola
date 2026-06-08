package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type EmailSuite struct {
	suite.Suite
}

func TestEmailSuite(t *testing.T) {
	suite.Run(t, new(EmailSuite))
}

func (s *EmailSuite) SetupTest() {}

func (s *EmailSuite) mustEmail(raw string) valueobjects.Email {
	email, err := valueobjects.NewEmail(raw)
	s.Require().NoError(err)
	return email
}

func (s *EmailSuite) TestNewEmail() {
	type args struct {
		input string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(valueobjects.Email, error)
	}{
		{
			name: "deve normalizar email valido em lowercase",
			args: args{input: "JOAO@EXAMPLE.COM"},
			expect: func(email valueobjects.Email, err error) {
				s.Require().NoError(err)
				s.Equal("joao@example.com", email.String())
			},
		},
		{
			name: "deve remover espacos externos",
			args: args{input: "  joao@example.com  "},
			expect: func(email valueobjects.Email, err error) {
				s.Require().NoError(err)
				s.Equal("joao@example.com", email.String())
			},
		},
		{
			name: "deve retornar erro para email vazio",
			args: args{input: ""},
			expect: func(email valueobjects.Email, err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, valueobjects.ErrEmailInvalid)
				s.Equal(valueobjects.Email{}, email)
			},
		},
		{
			name: "deve retornar erro para formato invalido",
			args: args{input: "@@example.com"},
			expect: func(email valueobjects.Email, err error) {
				s.Require().Error(err)
				s.True(errors.Is(err, valueobjects.ErrEmailInvalid))
				s.Equal(valueobjects.Email{}, email)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			email, err := valueobjects.NewEmail(scenario.args.input)
			scenario.expect(email, err)
		})
	}
}

func (s *EmailSuite) TestEqual() {
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
			name: "deve considerar emails equivalentes apos normalizacao",
			args: args{left: "JOAO@EXAMPLE.COM", right: "joao@example.com"},
			expect: func(equal bool) {
				s.True(equal)
			},
		},
		{
			name: "deve diferenciar emails distintos",
			args: args{left: "joao@example.com", right: "maria@example.com"},
			expect: func(equal bool) {
				s.False(equal)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			left := s.mustEmail(scenario.args.left)
			right := s.mustEmail(scenario.args.right)
			scenario.expect(left.Equal(right))
		})
	}
}

func (s *EmailSuite) TestMasked() {
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
			name: "deve mascarar email preenchido",
			args: args{input: "joao@example.com"},
			expect: func(masked string) {
				s.Equal("j***@example.com", masked)
			},
		},
		{
			name: "deve mascarar zero value",
			args: args{useZero: true},
			expect: func(masked string) {
				s.Equal("***", masked)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var email valueobjects.Email
			if !scenario.args.useZero {
				email = s.mustEmail(scenario.args.input)
			}

			scenario.expect(email.Masked())
		})
	}
}
