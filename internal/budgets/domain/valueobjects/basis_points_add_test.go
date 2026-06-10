package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type BasisPointsAddSuite struct {
	suite.Suite
}

func TestBasisPointsAddSuite(t *testing.T) {
	suite.Run(t, new(BasisPointsAddSuite))
}

func (s *BasisPointsAddSuite) TestAdd() {
	a, _ := valueobjects.NewBasisPoints(3000)
	b, _ := valueobjects.NewBasisPoints(7000)
	c, err := a.Add(b)
	s.NoError(err)
	s.Equal(10000, c.Int())
}

func (s *BasisPointsAddSuite) TestAddOverflow() {
	a, _ := valueobjects.NewBasisPoints(9000)
	b, _ := valueobjects.NewBasisPoints(2000)
	_, err := a.Add(b)
	s.ErrorIs(err, valueobjects.ErrBasisPointsOutOfRange)
}

func (s *BasisPointsAddSuite) TestCentsFromInt64() {
	c := valueobjects.CentsFromInt64(0)
	s.Equal(int64(0), c.Int64())
	s.False(c.IsPositive())
}
