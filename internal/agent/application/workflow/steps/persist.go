package steps

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PersistResult struct {
	AmountCents  int64
	CategoryPath string
	CardFound    bool
	CardName     string
}

type PersistFunc func(ctx context.Context, state ExpenseState) (PersistResult, error)

type persistStep struct {
	persist PersistFunc
}

func NewPersist(persist PersistFunc) platform.Step[ExpenseState] {
	return &persistStep{persist: persist}
}

func (s *persistStep) ID() string { return "persist" }

func (s *persistStep) Execute(ctx context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	result, err := s.persist(ctx, state)
	if err != nil {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusFailed}, err
	}
	state.AmountCents = result.AmountCents
	state.CategoryPath = result.CategoryPath
	state.CardName = result.CardName
	state.Outcome = tools.OutcomeRouted
	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
}
