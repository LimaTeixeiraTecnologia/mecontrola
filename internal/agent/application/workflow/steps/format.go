package steps

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type FormatFunc func(state ExpenseState) string

type formatStep struct {
	format FormatFunc
}

func NewFormat(format FormatFunc) platform.Step[ExpenseState] {
	return &formatStep{format: format}
}

func (s *formatStep) ID() string { return "format" }

func (s *formatStep) Execute(_ context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if state.Outcome != tools.OutcomeRouted {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Reply = s.format(state)
	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
}
