package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type PendingEventOutcomeKind int

const (
	OutcomeReject PendingEventOutcomeKind = iota + 1
	OutcomeNoop
	OutcomeCreate
	OutcomeUpdate
	OutcomeDelete
	OutcomeDefer
)

type PendingEventOutcome struct {
	Kind            PendingEventOutcomeKind
	ExpectedVersion int64
}

type PendingEventOutcomeResolver struct{}

func NewPendingEventOutcomeResolver() *PendingEventOutcomeResolver {
	return &PendingEventOutcomeResolver{}
}

func (r *PendingEventOutcomeResolver) Decide(event entities.PendingEvent, current *entities.Expense) PendingEventOutcome {
	if event.MutationKind() == valueobjects.MutationKindCreate {
		if current != nil {
			return PendingEventOutcome{Kind: OutcomeNoop}
		}
		if event.ExpectedVersion() != 1 {
			return PendingEventOutcome{Kind: OutcomeNoop}
		}
		return PendingEventOutcome{Kind: OutcomeCreate}
	}

	if current == nil {
		return PendingEventOutcome{Kind: OutcomeDefer}
	}

	currentVersion := current.Version()

	if event.ExpectedVersion() <= currentVersion {
		return PendingEventOutcome{Kind: OutcomeNoop}
	}

	if event.ExpectedVersion() > currentVersion+1 {
		return PendingEventOutcome{Kind: OutcomeDefer}
	}

	if event.MutationKind() == valueobjects.MutationKindDelete {
		return PendingEventOutcome{Kind: OutcomeDelete, ExpectedVersion: currentVersion}
	}

	return PendingEventOutcome{Kind: OutcomeUpdate, ExpectedVersion: currentVersion}
}
