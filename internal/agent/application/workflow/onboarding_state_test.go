package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type OnboardingStateSuite struct {
	suite.Suite
}

func TestOnboardingStateSuite(t *testing.T) {
	suite.Run(t, new(OnboardingStateSuite))
}

func (s *OnboardingStateSuite) TestOnboardingAwaitingString() {
	s.Equal("none", workflow.AwaitingNone.String())
	s.Equal("text", workflow.AwaitingText.String())
	s.Equal("confirm", workflow.AwaitingConfirm.String())
	s.Equal("unknown", workflow.OnboardingAwaiting(0).String())
}

func (s *OnboardingStateSuite) TestOnboardingAwaitingIsValid() {
	s.True(workflow.AwaitingNone.IsValid())
	s.True(workflow.AwaitingConfirm.IsValid())
	s.False(workflow.OnboardingAwaiting(0).IsValid())
	s.False(workflow.OnboardingAwaiting(99).IsValid())
}

func (s *OnboardingStateSuite) TestCorrectionTargetString() {
	s.Equal("none", workflow.CorrectionTargetNone.String())
	s.Equal("objective", workflow.CorrectionTargetObjective.String())
	s.Equal("budget", workflow.CorrectionTargetBudget.String())
	s.Equal("cards", workflow.CorrectionTargetCards.String())
	s.Equal("values", workflow.CorrectionTargetValues.String())
	s.Equal("unknown", workflow.CorrectionTarget(0).String())
}

func (s *OnboardingStateSuite) TestCorrectionTargetIsValid() {
	s.True(workflow.CorrectionTargetNone.IsValid())
	s.True(workflow.CorrectionTargetValues.IsValid())
	s.False(workflow.CorrectionTarget(0).IsValid())
	s.False(workflow.CorrectionTarget(99).IsValid())
}

func (s *OnboardingStateSuite) TestOnboardingStateDefaults() {
	state := workflow.OnboardingState{
		Phase:    valueobjects.PhaseWelcome,
		Awaiting: workflow.AwaitingNone,
	}
	s.Equal(valueobjects.PhaseWelcome, state.Phase)
	s.Equal(workflow.AwaitingNone, state.Awaiting)
	s.Empty(state.Inbound)
}
