package steps

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type AuthorizeFunc func(ctx context.Context, state ExpenseState) bool

type authorizeStep struct {
	authorize AuthorizeFunc
	denyReply string
}

func NewAuthorize(authorize AuthorizeFunc, denyReply string) platform.Step[ExpenseState] {
	return &authorizeStep{authorize: authorize, denyReply: denyReply}
}

func (s *authorizeStep) ID() string { return "authorize" }

func (s *authorizeStep) Execute(ctx context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if s.authorize(ctx, state) {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Outcome = tools.OutcomeAuthzDenied
	state.Reply = s.denyReply
	state.ShortCircuit = true
	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
}
