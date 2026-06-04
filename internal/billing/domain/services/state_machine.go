package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

const DefaultGracePeriod = 7 * 24 * time.Hour

type StateMachine struct{}

func NewStateMachine() StateMachine { return StateMachine{} }

func (s StateMachine) AssertLegal(from, to valueobjects.SubscriptionStatus) error {
	if isLegalTransition(from, to) {
		return nil
	}
	return ErrIllegalTransition
}

func isLegalTransition(from, to valueobjects.SubscriptionStatus) bool {
	switch from {
	case valueobjects.SubscriptionStatusTrialing:
		return to == valueobjects.SubscriptionStatusActive ||
			to == valueobjects.SubscriptionStatusExpired
	case valueobjects.SubscriptionStatusActive:
		return to == valueobjects.SubscriptionStatusPastDue ||
			to == valueobjects.SubscriptionStatusCanceledPending ||
			to == valueobjects.SubscriptionStatusRefunded
	case valueobjects.SubscriptionStatusPastDue:
		return to == valueobjects.SubscriptionStatusActive ||
			to == valueobjects.SubscriptionStatusExpired ||
			to == valueobjects.SubscriptionStatusRefunded
	case valueobjects.SubscriptionStatusCanceledPending:
		return to == valueobjects.SubscriptionStatusExpired ||
			to == valueobjects.SubscriptionStatusActive ||
			to == valueobjects.SubscriptionStatusRefunded
	case valueobjects.SubscriptionStatusExpired:
		return false
	case valueobjects.SubscriptionStatusRefunded:
		return false
	default:
		return false
	}
}
