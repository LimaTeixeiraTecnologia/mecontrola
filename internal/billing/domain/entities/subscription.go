package entities

import (
	"errors"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

var ErrOccurredAtRequired = errors.New("billing: occurred_at is required")

var ErrTransitionNotAllowed = errors.New("billing: transition not allowed")

type Subscription struct {
	id          string
	userID      string
	plan        valueobjects.Plan
	funnelToken valueobjects.FunnelToken
	status      valueobjects.Status
	periodStart time.Time
	periodEnd   time.Time
	graceEnd    time.Time
	lastEventAt time.Time
}

func NewSubscription(plan valueobjects.Plan, funnelToken valueobjects.FunnelToken) Subscription {
	return Subscription{
		plan:        plan,
		funnelToken: funnelToken,
	}
}

func Hydrate(
	id string,
	funnelToken valueobjects.FunnelToken,
	plan valueobjects.Plan,
	status valueobjects.Status,
	periodStart time.Time,
	periodEnd time.Time,
	graceEnd time.Time,
	lastEventAt time.Time,
) Subscription {
	return Subscription{
		id:          id,
		plan:        plan,
		funnelToken: funnelToken,
		status:      status,
		periodStart: periodStart,
		periodEnd:   periodEnd,
		graceEnd:    graceEnd,
		lastEventAt: lastEventAt,
	}
}

func HydrateWithUser(
	id string,
	userID string,
	funnelToken valueobjects.FunnelToken,
	plan valueobjects.Plan,
	status valueobjects.Status,
	periodStart time.Time,
	periodEnd time.Time,
	graceEnd time.Time,
	lastEventAt time.Time,
) Subscription {
	return Subscription{
		id:          id,
		userID:      userID,
		plan:        plan,
		funnelToken: funnelToken,
		status:      status,
		periodStart: periodStart,
		periodEnd:   periodEnd,
		graceEnd:    graceEnd,
		lastEventAt: lastEventAt,
	}
}

func (s Subscription) ID() string {
	return s.id
}

func (s Subscription) UserID() string {
	return s.userID
}

func (s Subscription) PeriodStart() time.Time {
	return s.periodStart
}

func (s Subscription) FunnelToken() valueobjects.FunnelToken {
	return s.funnelToken
}

func (s Subscription) GraceEnd() time.Time {
	return s.graceEnd
}

func (s Subscription) LastEventAt() time.Time {
	return s.lastEventAt
}

func (s Subscription) PeriodEnd() time.Time {
	return s.periodEnd
}

func (s Subscription) Plan() valueobjects.Plan {
	return s.plan
}

func (s Subscription) Status() valueobjects.Status {
	return s.status
}

func (s *Subscription) Activate(occurredAt time.Time) error {
	return s.applyStatusTransition(services.TriggerSaleApproved, occurredAt, 0)
}

func (s *Subscription) MarkCanceled(occurredAt time.Time) error {
	return s.applyStatusTransition(services.TriggerSubscriptionCanceled, occurredAt, 0)
}

func (s *Subscription) MarkPastDue(occurredAt time.Time, graceDuration time.Duration) error {
	return s.applyStatusTransition(services.TriggerSubscriptionLate, occurredAt, graceDuration)
}

func (s *Subscription) MarkRefunded(occurredAt time.Time) error {
	return s.applyStatusTransition(services.TriggerRefunded, occurredAt, 0)
}

func (s *Subscription) Renew(occurredAt time.Time) error {
	return s.applyStatusTransition(services.TriggerSubscriptionRenewed, occurredAt, 0)
}

func (s *Subscription) MarkExpiredAfterGrace(occurredAt time.Time) error {
	return s.applyStatusTransition(services.TriggerGraceExpired, occurredAt, 0)
}

func (s *Subscription) applyActive(occurredAt time.Time) {
	base := occurredAt
	if s.periodEnd.After(occurredAt) {
		base = s.periodEnd
	}

	s.status = valueobjects.StatusActive
	s.periodEnd = base.Add(s.plan.Duration())
	s.graceEnd = time.Time{}
	s.lastEventAt = occurredAt
}

func (s *Subscription) applyCanceled(occurredAt time.Time) {
	s.status = valueobjects.StatusCanceledPending
	s.graceEnd = time.Time{}
	s.lastEventAt = occurredAt
}

func (s *Subscription) applyPastDue(occurredAt time.Time, graceDuration time.Duration) {
	s.status = valueobjects.StatusPastDue
	s.graceEnd = occurredAt.Add(graceDuration)
	s.lastEventAt = occurredAt
}

func (s *Subscription) applyExpired(occurredAt time.Time) {
	s.status = valueobjects.StatusExpired
	s.graceEnd = time.Time{}
	s.lastEventAt = occurredAt
}

func (s *Subscription) applyRefunded(occurredAt time.Time) {
	if s.status == valueobjects.StatusRefunded && !occurredAt.After(s.lastEventAt) {
		return
	}

	s.status = valueobjects.StatusRefunded
	s.graceEnd = time.Time{}
	s.lastEventAt = occurredAt
}

func (s *Subscription) applyStatusTransition(
	trigger services.Trigger,
	occurredAt time.Time,
	graceDuration time.Duration,
) error {
	if occurredAt.IsZero() {
		return ErrOccurredAtRequired
	}

	transitionService := services.NewTransitionService()
	targetStatus, ok := transitionService.TargetStatus(s.status, trigger)
	if !ok {
		return fmt.Errorf("billing: %s -> trigger %d: %w", s.status.String(), trigger, ErrTransitionNotAllowed)
	}

	switch targetStatus {
	case valueobjects.StatusActive:
		s.applyActive(occurredAt)
	case valueobjects.StatusPastDue:
		s.applyPastDue(occurredAt, graceDuration)
	case valueobjects.StatusCanceledPending:
		s.applyCanceled(occurredAt)
	case valueobjects.StatusRefunded:
		s.applyRefunded(occurredAt)
	case valueobjects.StatusExpired:
		s.applyExpired(occurredAt)
	default:
		return fmt.Errorf("billing: %s -> %s: %w", s.status.String(), targetStatus.String(), ErrTransitionNotAllowed)
	}

	return nil
}
