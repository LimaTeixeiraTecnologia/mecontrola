package valueobjects

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfidenceSuite struct {
	suite.Suite
	ctx context.Context
}

func TestConfidenceSuite(t *testing.T) {
	suite.Run(t, new(ConfidenceSuite))
}

func (s *ConfidenceSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ConfidenceSuite) TestNewConfidence() {
	type args struct {
		raw float64
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(value Confidence, err error)
	}{
		{
			name: "zero é válido",
			args: args{raw: 0},
			expect: func(value Confidence, err error) {
				s.NoError(err)
				s.InDelta(0, value.Value(), 1e-9)
			},
		},
		{
			name: "um é válido",
			args: args{raw: 1},
			expect: func(value Confidence, err error) {
				s.NoError(err)
				s.InDelta(1, value.Value(), 1e-9)
			},
		},
		{
			name: "intermediário é válido",
			args: args{raw: 0.8},
			expect: func(value Confidence, err error) {
				s.NoError(err)
				s.InDelta(0.8, value.Value(), 1e-9)
			},
		},
		{
			name: "negativo é inválido",
			args: args{raw: -0.01},
			expect: func(value Confidence, err error) {
				s.ErrorIs(err, ErrConfidenceOutOfRange)
				s.InDelta(0, value.Value(), 1e-9)
			},
		},
		{
			name: "acima de um é inválido",
			args: args{raw: 1.01},
			expect: func(value Confidence, err error) {
				s.ErrorIs(err, ErrConfidenceOutOfRange)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			value, err := NewConfidence(scenario.args.raw)
			scenario.expect(value, err)
		})
	}
}

func (s *ConfidenceSuite) TestBelow() {
	type args struct {
		value     float64
		threshold float64
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(below bool)
	}{
		{
			name:   "abaixo do threshold",
			args:   args{value: 0.79, threshold: 0.8},
			expect: func(below bool) { s.True(below) },
		},
		{
			name:   "igual ao threshold não é below",
			args:   args{value: 0.8, threshold: 0.8},
			expect: func(below bool) { s.False(below) },
		},
		{
			name:   "acima do threshold",
			args:   args{value: 0.9, threshold: 0.8},
			expect: func(below bool) { s.False(below) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			value, err := NewConfidence(scenario.args.value)
			s.Require().NoError(err)
			threshold, err := NewConfidence(scenario.args.threshold)
			s.Require().NoError(err)
			scenario.expect(value.Below(threshold))
		})
	}
}
