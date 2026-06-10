package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CentsSuite struct {
	suite.Suite
}

func TestCentsSuite(t *testing.T) {
	suite.Run(t, new(CentsSuite))
}

func (s *CentsSuite) TestNewCents() {
	type testCase struct {
		name    string
		input   int64
		wantErr bool
	}

	cases := []testCase{
		{name: "positivo", input: 100, wantErr: false},
		{name: "um centavo", input: 1, wantErr: false},
		{name: "zero", input: 0, wantErr: true},
		{name: "negativo", input: -1, wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewCents(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.input, got.Int64())
			s.True(got.IsPositive())
		})
	}
}

func (s *CentsSuite) TestAdd() {
	a, _ := valueobjects.NewCents(100)
	b, _ := valueobjects.NewCents(50)
	c := a.Add(b)
	s.Equal(int64(150), c.Int64())
}
