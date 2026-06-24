package steps

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type ReplayFunc func(ctx context.Context, state ExpenseState) (reply string, found bool)

type replayStep struct {
	replay ReplayFunc
}

func NewReplay(replay ReplayFunc) platform.Step[ExpenseState] {
	return &replayStep{replay: replay}
}

func (s *replayStep) ID() string { return "replay" }

func (s *replayStep) Execute(ctx context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	reply, found := s.replay(ctx, state)
	if !found {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Outcome = tools.OutcomeReplay
	state.Reply = reply
	state.ShortCircuit = true
	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
}
