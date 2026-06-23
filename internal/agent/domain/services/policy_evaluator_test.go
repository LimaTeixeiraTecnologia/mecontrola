package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type PolicyEvaluatorSuite struct {
	suite.Suite
	ctx context.Context
}

func TestPolicyEvaluatorSuite(t *testing.T) {
	suite.Run(t, new(PolicyEvaluatorSuite))
}

func (s *PolicyEvaluatorSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *PolicyEvaluatorSuite) TestEvaluate() {
	type args struct {
		kind       intent.Kind
		confidence float64
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision PolicyDecision)
	}{
		{
			name:   "write abaixo do threshold pede esclarecimento",
			args:   args{kind: intent.KindRecordExpense, confidence: 0.5},
			expect: func(decision PolicyDecision) { s.Equal(PolicyDecisionClarify, decision) },
		},
		{
			name:   "write no threshold prossegue",
			args:   args{kind: intent.KindRecordExpense, confidence: 0.8},
			expect: func(decision PolicyDecision) { s.Equal(PolicyDecisionProceed, decision) },
		},
		{
			name:   "write acima do threshold prossegue",
			args:   args{kind: intent.KindCreateCard, confidence: 0.95},
			expect: func(decision PolicyDecision) { s.Equal(PolicyDecisionProceed, decision) },
		},
		{
			name:   "read abaixo do threshold prossegue",
			args:   args{kind: intent.KindMonthlySummary, confidence: 0.1},
			expect: func(decision PolicyDecision) { s.Equal(PolicyDecisionProceed, decision) },
		},
		{
			name:   "configure_budget é write e respeita threshold",
			args:   args{kind: intent.KindConfigureBudget, confidence: 0.3},
			expect: func(decision PolicyDecision) { s.Equal(PolicyDecisionClarify, decision) },
		},
		{
			name:   "unknown não é write e prossegue",
			args:   args{kind: intent.KindUnknown, confidence: 0},
			expect: func(decision PolicyDecision) { s.Equal(PolicyDecisionProceed, decision) },
		},
	}

	min, err := valueobjects.NewConfidence(0.8)
	s.Require().NoError(err)
	evaluator := NewPolicyEvaluator(min)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			confidence, confErr := valueobjects.NewConfidence(scenario.args.confidence)
			s.Require().NoError(confErr)
			scenario.expect(evaluator.Evaluate(scenario.args.kind, confidence))
		})
	}
}
