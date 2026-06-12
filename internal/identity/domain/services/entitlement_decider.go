package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
)

type EntitlementDecider struct{}

type EntitlementDecision struct {
	Entitled bool
	Reason   domain.Reason
}

func (EntitlementDecider) Decide(sub domain.Subscription, now time.Time) EntitlementDecision {
	entitled, reason := domain.IsEntitled(sub, now)
	return EntitlementDecision{Entitled: entitled, Reason: reason}
}
