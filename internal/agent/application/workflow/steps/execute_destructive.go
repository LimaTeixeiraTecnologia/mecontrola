package steps

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DestructiveExecutor func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error)

type ExecuteDestructiveDeps struct {
	Executors map[confirmation.OperationKind]DestructiveExecutor
}

type executeDestructiveStep struct {
	executors map[confirmation.OperationKind]DestructiveExecutor
}

func NewExecuteDestructive(deps ExecuteDestructiveDeps) platform.Step[confirmation.ConfirmState] {
	return &executeDestructiveStep{executors: deps.Executors}
}

func (s *executeDestructiveStep) ID() string { return "execute_destructive" }

func (s *executeDestructiveStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	executor, ok := s.executors[state.OperationKind]
	if !ok {
		state.ShortCircuit = true
		state.Reply = fmt.Sprintf("Não sei como executar a operação '%s'.", state.OperationKind.String())
		state.Outcome = int(tools.OutcomeMissingResolver)
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	result, err := executor(ctx, state)
	if err != nil {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("execute_destructive: %w", err)
	}

	if result.Outcome == 0 {
		result.Outcome = int(tools.OutcomeRouted)
	}

	return platform.StepOutput[confirmation.ConfirmState]{State: result, Status: platform.StepStatusCompleted}, nil
}
