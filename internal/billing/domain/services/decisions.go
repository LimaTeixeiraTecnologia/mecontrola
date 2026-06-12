package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type Decision uint8

const (
	DecisionApply Decision = iota + 1
	DecisionSkipAsRegression
)

func (s TransitionService) DecideRenewal(current valueobjects.Status, occurredAt, lastEventAt time.Time) Decision {
	return s.decideFor(current, TriggerSubscriptionRenewed, occurredAt, lastEventAt)
}

func (s TransitionService) DecidePastDue(current valueobjects.Status, occurredAt, lastEventAt time.Time) Decision {
	return s.decideFor(current, TriggerSubscriptionLate, occurredAt, lastEventAt)
}

func (s TransitionService) DecideCancellation(current valueobjects.Status, occurredAt, lastEventAt time.Time) Decision {
	return s.decideFor(current, TriggerSubscriptionCanceled, occurredAt, lastEventAt)
}

func (s TransitionService) decideFor(current valueobjects.Status, trigger Trigger, occurredAt, lastEventAt time.Time) Decision {
	if s.IsRegression(current, trigger, occurredAt, lastEventAt) {
		return DecisionSkipAsRegression
	}
	return DecisionApply
}
