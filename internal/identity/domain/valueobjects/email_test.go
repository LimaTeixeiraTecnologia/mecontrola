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

func TestEmail(t *testing.T) {
	suite.Run(t, new(EmailSuite))
}

func (s *EmailSuite) TestValidInputs() {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "email minusculo", input: "user@example.com", expected: "user@example.com"},
		{name: "email maiusculo", input: "USER@EXAMPLE.COM", expected: "user@example.com"},
		{name: "email mixed case", input: "User.Name@Example.Com", expected: "user.name@example.com"},
		{name: "email com subdominio", input: "user@mail.example.com", expected: "user@mail.example.com"},
		{name: "email com pontos", input: "first.last@domain.org", expected: "first.last@domain.org"},
		{name: "email com plus", input: "user+tag@example.com", expected: "user+tag@example.com"},
		{name: "email com espacos nas bordas", input: "  user@example.com  ", expected: "user@example.com"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewEmail(tc.input)
			s.NoError(err)
			s.Equal(tc.expected, got.String())
		})
	}
}

func (s *EmailSuite) TestInvalidInputs() {
	cases := []struct {
		name        string
		input       string
		expectedErr error
	}{
		{name: "vazio", input: "", expectedErr: valueobjects.ErrEmptyEmail},
		{name: "so espacos", input: "   ", expectedErr: valueobjects.ErrEmptyEmail},
		{name: "sem arroba", input: "userexample.com", expectedErr: valueobjects.ErrInvalidEmail},
		{name: "sem TLD", input: "user@example", expectedErr: valueobjects.ErrInvalidEmail},
		{name: "dominio sem ponto", input: "user@localhost", expectedErr: valueobjects.ErrInvalidEmail},
		{name: "so arroba", input: "@", expectedErr: valueobjects.ErrInvalidEmail},
		{name: "arroba no inicio", input: "@example.com", expectedErr: valueobjects.ErrInvalidEmail},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := valueobjects.NewEmail(tc.input)
			s.Error(err)
			s.True(errors.Is(err, tc.expectedErr), "esperado %v, got %v", tc.expectedErr, err)
		})
	}
}

func (s *EmailSuite) TestIsZero() {
	var e valueobjects.Email
	s.True(e.IsZero())

	e2, err := valueobjects.NewEmail("user@example.com")
	s.NoError(err)
	s.False(e2.IsZero())
}

func (s *EmailSuite) TestEquals() {
	a, _ := valueobjects.NewEmail("user@example.com")
	b, _ := valueobjects.NewEmail("USER@EXAMPLE.COM")
	c, _ := valueobjects.NewEmail("other@example.com")

	s.True(a.Equals(b))
	s.False(a.Equals(c))
}

func (s *EmailSuite) TestLowercaseNormalization() {
	got, err := valueobjects.NewEmail("ADMIN@DOMAIN.COM")
	s.NoError(err)
	s.Equal("admin@domain.com", got.String())
}
