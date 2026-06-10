package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type BasisPointsSuite struct {
	suite.Suite
}

func TestBasisPointsSuite(t *testing.T) {
	suite.Run(t, new(BasisPointsSuite))
}

func (s *BasisPointsSuite) TestNewBasisPoints() {
	type testCase struct {
		name    string
		input   int
		wantErr bool
	}

	cases := []testCase{
		{name: "zero", input: 0, wantErr: false},
		{name: "máximo", input: 10000, wantErr: false},
		{name: "meio", input: 5000, wantErr: false},
		{name: "abaixo de zero", input: -1, wantErr: true},
		{name: "acima de 10000", input: 10001, wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewBasisPoints(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.input, got.Int())
		})
	}
}
