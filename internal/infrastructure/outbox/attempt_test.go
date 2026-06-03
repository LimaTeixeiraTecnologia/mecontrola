package outbox_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

type AttemptSuite struct {
	suite.Suite
}

func TestAttempt(t *testing.T) {
	suite.Run(t, new(AttemptSuite))
}

func (s *AttemptSuite) TestNewAttempt() {
	a := outbox.NewAttempt(5)
	s.Equal(uint8(5), a.Value())
}

func (s *AttemptSuite) TestNext() {
	scenarios := []struct {
		name   string
		input  uint8
		expect uint8
	}{
		{"incrementa de 0", 0, 1},
		{"incrementa de 1", 1, 2},
		{"incrementa de 14", 14, 15},
		{"nao faz overflow em 255", 255, 255},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			a := outbox.NewAttempt(sc.input)
			s.Equal(sc.expect, a.Next().Value())
		})
	}
}

func (s *AttemptSuite) TestIsExhausted() {
	max := outbox.NewAttempt(15)
	scenarios := []struct {
		name    string
		current uint8
		expect  bool
	}{
		{"0 nao esta esgotado", 0, false},
		{"14 nao esta esgotado", 14, false},
		{"15 esta esgotado", 15, true},
		{"20 esta esgotado", 20, true},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			a := outbox.NewAttempt(sc.current)
			s.Equal(sc.expect, a.IsExhausted(max))
		})
	}
}
