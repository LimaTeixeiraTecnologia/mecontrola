package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ProducerSourceSuite struct {
	suite.Suite
}

func TestProducerSourceSuite(t *testing.T) {
	suite.Run(t, new(ProducerSourceSuite))
}

func (s *ProducerSourceSuite) TestNewProducerSource() {
	type testCase struct {
		name    string
		input   string
		wantErr bool
	}

	cases := []testCase{
		{name: "api", input: "api", wantErr: false},
		{name: "outro produtor", input: "billing", wantErr: false},
		{name: "vazio", input: "", wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewProducerSource(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.input, got.String())
		})
	}
}

func (s *ProducerSourceSuite) TestEqual() {
	a, _ := valueobjects.NewProducerSource("api")
	b, _ := valueobjects.NewProducerSource("api")
	c, _ := valueobjects.NewProducerSource("billing")
	s.True(a.Equal(b))
	s.False(a.Equal(c))
}
