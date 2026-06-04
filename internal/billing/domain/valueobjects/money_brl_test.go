package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type MoneyBRLSuite struct {
	suite.Suite
}

func TestMoneyBRL(t *testing.T) {
	suite.Run(t, new(MoneyBRLSuite))
}

func (s *MoneyBRLSuite) TestZero() {
	m, err := valueobjects.NewMoneyBRL(0)
	s.NoError(err)
	s.True(m.IsZero())
	s.Equal(int64(0), m.Cents())
}

func (s *MoneyBRLSuite) TestPositive() {
	cases := []int64{1, 100, 2990, 29780, 1_000_000}
	for _, cents := range cases {
		s.Run("", func() {
			m, err := valueobjects.NewMoneyBRL(cents)
			s.NoError(err)
			s.Equal(cents, m.Cents())
			s.False(m.IsZero())
		})
	}
}

func (s *MoneyBRLSuite) TestNegativeReturnsError() {
	cases := []int64{-1, -100, -999_999}
	for _, cents := range cases {
		s.Run("", func() {
			_, err := valueobjects.NewMoneyBRL(cents)
			s.True(errors.Is(err, valueobjects.ErrNegativeAmount))
		})
	}
}
