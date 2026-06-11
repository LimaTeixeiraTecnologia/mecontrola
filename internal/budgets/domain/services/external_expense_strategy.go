package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExternalExpenseAction int

const (
	ActionCreate ExternalExpenseAction = iota + 1
	ActionUpdate
	ActionDelete
	ActionQueuePending
	ActionNoop
)

type ExternalExpenseStrategy struct{}

func NewExternalExpenseStrategy() *ExternalExpenseStrategy {
	return &ExternalExpenseStrategy{}
}

func (s *ExternalExpenseStrategy) Plan(
	mutationKind valueobjects.MutationKind,
	currentVersion int64,
	currentExists bool,
	eventVersion int64,
) ExternalExpenseAction {
	if mutationKind == valueobjects.MutationKindCreate {
		if currentExists {
			return ActionNoop
		}
		if eventVersion != 1 {
			return ActionQueuePending
		}
		return ActionCreate
	}

	if !currentExists {
		return ActionQueuePending
	}

	expectedPrevious := eventVersion - 1
	if expectedPrevious < currentVersion {
		return ActionNoop
	}
	if expectedPrevious > currentVersion {
		return ActionQueuePending
	}

	if mutationKind == valueobjects.MutationKindDelete {
		return ActionDelete
	}
	return ActionUpdate
}
