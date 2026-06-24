package steps

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PolicyFunc func(ctx context.Context, state ExpenseState) (blocked bool, reply string)

type policyStep struct {
	policy PolicyFunc
}

func NewPolicy(policy PolicyFunc) platform.Step[ExpenseState] {
	return &policyStep{policy: policy}
}

func (s *policyStep) ID() string { return "policy" }

func (s *policyStep) Execute(ctx context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	blocked, reply := s.policy(ctx, state)
	if !blocked {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Outcome = tools.OutcomePolicyBlocked
	state.Reply = reply
	state.ShortCircuit = true
	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
}
