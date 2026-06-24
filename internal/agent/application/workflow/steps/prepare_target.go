package steps

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type TargetResolver func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error)

type PrepareTargetDeps struct {
	Targets map[confirmation.OperationKind]TargetResolver
}

type prepareTargetStep struct {
	targets map[confirmation.OperationKind]TargetResolver
}

func NewPrepareTarget(deps PrepareTargetDeps) platform.Step[confirmation.ConfirmState] {
	return &prepareTargetStep{targets: deps.Targets}
}

func (s *prepareTargetStep) ID() string { return "prepare_target" }

func (s *prepareTargetStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	resolver, ok := s.targets[state.OperationKind]
	if !ok {
		state.ShortCircuit = true
		state.Reply = fmt.Sprintf("Não sei como resolver a operação '%s'.", state.OperationKind.String())
		state.Outcome = int(tools.OutcomeMissingResolver)
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	resolved, err := resolver(ctx, state)
	if err != nil {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("prepare_target: %w", err)
	}

	return platform.StepOutput[confirmation.ConfirmState]{State: resolved, Status: platform.StepStatusCompleted}, nil
}
