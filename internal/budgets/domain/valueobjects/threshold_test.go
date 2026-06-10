package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdSuite struct {
	suite.Suite
}

func TestThresholdSuite(t *testing.T) {
	suite.Run(t, new(ThresholdSuite))
}

func (s *ThresholdSuite) TestParseThreshold() {
	type testCase struct {
		name    string
		input   int
		want    valueobjects.Threshold
		wantErr bool
	}

	cases := []testCase{
		{name: "80", input: 80, want: valueobjects.Threshold80, wantErr: false},
		{name: "100", input: 100, want: valueobjects.Threshold100, wantErr: false},
		{name: "50", input: 50, wantErr: true},
		{name: "0", input: 0, wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.ParseThreshold(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.want, got)
			s.Equal(tc.input, got.Int())
		})
	}
}

func (s *ThresholdSuite) TestString() {
	s.Equal("t80", valueobjects.Threshold80.String())
	s.Equal("t100", valueobjects.Threshold100.String())
}
