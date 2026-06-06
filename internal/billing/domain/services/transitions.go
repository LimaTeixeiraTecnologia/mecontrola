package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type Trigger uint8

const (
	TriggerSaleApproved Trigger = iota + 1
	TriggerSubscriptionRenewed
	TriggerSubscriptionLate
	TriggerSubscriptionCanceled
	TriggerRefunded
)

type TransitionService struct{}

func NewTransitionService() TransitionService {
	return TransitionService{}
}

func (s TransitionService) CanTransition(current, next valueobjects.Status) bool {
	table := s.transitionTable()
	if int(current) >= len(table) || int(next) >= len(table[current]) {
		return false
	}

	return table[current][next]
}

func (s TransitionService) IsRegression(
	currentStatus valueobjects.Status,
	incomingTrigger Trigger,
	occurredAt time.Time,
	lastEventAt time.Time,
) bool {
	if incomingTrigger == TriggerRefunded {
		return false
	}
	if lastEventAt.IsZero() || occurredAt.After(lastEventAt) {
		return false
	}

	targetStatus, ok := s.TargetStatus(currentStatus, incomingTrigger)
	if !ok {
		return false
	}

	return targetStatus != currentStatus
}

func (s TransitionService) TargetStatus(current valueobjects.Status, trigger Trigger) (valueobjects.Status, bool) {
	if current == 0 {
		switch trigger {
		case TriggerSaleApproved, TriggerSubscriptionRenewed:
			return valueobjects.StatusActive, true
		default:
			return 0, false
		}
	}

	switch trigger {
	case TriggerSaleApproved, TriggerSubscriptionRenewed:
		if s.CanTransition(current, valueobjects.StatusActive) {
			return valueobjects.StatusActive, true
		}
	case TriggerSubscriptionLate:
		if s.CanTransition(current, valueobjects.StatusPastDue) {
			return valueobjects.StatusPastDue, true
		}
	case TriggerSubscriptionCanceled:
		if s.CanTransition(current, valueobjects.StatusCanceledPending) {
			return valueobjects.StatusCanceledPending, true
		}
	case TriggerRefunded:
		if current == valueobjects.StatusRefunded || s.CanTransition(current, valueobjects.StatusRefunded) {
			return valueobjects.StatusRefunded, true
		}
	}

	return 0, false
}

func (s TransitionService) transitionTable() [7][7]bool {
	return [7][7]bool{
		{},
		{},
		{
			valueobjects.StatusActive:          true,
			valueobjects.StatusPastDue:         true,
			valueobjects.StatusCanceledPending: true,
			valueobjects.StatusRefunded:        true,
		},
		{
			valueobjects.StatusActive:          true,
			valueobjects.StatusPastDue:         true,
			valueobjects.StatusCanceledPending: true,
			valueobjects.StatusRefunded:        true,
		},
		{
			valueobjects.StatusActive:   true,
			valueobjects.StatusRefunded: true,
		},
		{
			valueobjects.StatusActive:   true,
			valueobjects.StatusRefunded: true,
		},
		{},
	}
}
