package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type OnboardingPhaseSuite struct {
	suite.Suite
}

func TestOnboardingPhaseSuite(t *testing.T) {
	suite.Run(t, new(OnboardingPhaseSuite))
}

func (s *OnboardingPhaseSuite) TestString() {
	scenarios := []struct {
		name   string
		phase  valueobjects.OnboardingPhase
		expect string
	}{
		{name: "welcome", phase: valueobjects.PhaseWelcome, expect: "welcome"},
		{name: "objective", phase: valueobjects.PhaseObjective, expect: "objective"},
		{name: "budget", phase: valueobjects.PhaseBudget, expect: "budget"},
		{name: "cards", phase: valueobjects.PhaseCards, expect: "cards"},
		{name: "categories", phase: valueobjects.PhaseCategories, expect: "categories"},
		{name: "values", phase: valueobjects.PhaseValues, expect: "values"},
		{name: "summary", phase: valueobjects.PhaseSummary, expect: "summary"},
		{name: "conclusion", phase: valueobjects.PhaseConclusion, expect: "conclusion"},
		{name: "unknown", phase: valueobjects.OnboardingPhase(0), expect: "unknown"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, scenario.phase.String())
		})
	}
}

func (s *OnboardingPhaseSuite) TestIsValid() {
	s.True(valueobjects.PhaseWelcome.IsValid())
	s.True(valueobjects.PhaseConclusion.IsValid())
	s.False(valueobjects.OnboardingPhase(0).IsValid())
	s.False(valueobjects.OnboardingPhase(99).IsValid())
}

func (s *OnboardingPhaseSuite) TestParseOnboardingPhase() {
	type args struct {
		raw string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(valueobjects.OnboardingPhase, error)
	}{
		{
			name: "deve converter welcome",
			args: args{raw: "welcome"},
			expect: func(phase valueobjects.OnboardingPhase, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.PhaseWelcome, phase)
			},
		},
		{
			name: "deve converter conclusion",
			args: args{raw: "conclusion"},
			expect: func(phase valueobjects.OnboardingPhase, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.PhaseConclusion, phase)
			},
		},
		{
			name: "deve retornar erro para phase invalida",
			args: args{raw: "invalid"},
			expect: func(phase valueobjects.OnboardingPhase, err error) {
				s.ErrorIs(err, valueobjects.ErrOnboardingPhaseInvalid)
				s.Zero(phase)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			phase, err := valueobjects.ParseOnboardingPhase(scenario.args.raw)
			scenario.expect(phase, err)
		})
	}
}
