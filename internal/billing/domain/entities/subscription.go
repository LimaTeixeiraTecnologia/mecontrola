package entities

import (
	"time"

	billingdomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
)

type Subscription struct {
	id                 SubscriptionID
	userID             identityentities.UserID
	provider           string
	externalSubID      valueobjects.ExternalSubscriptionID
	planCode           valueobjects.PlanCode
	status             valueobjects.SubscriptionStatus
	period             valueobjects.BillingPeriod
	periodStart        time.Time
	periodEnd          time.Time
	gracePeriodEnd     time.Time
	refundAmountCents  valueobjects.MoneyBRL
	lastEventAt        time.Time
	lastWebhookEventID valueobjects.WebhookEventID
	createdAt          time.Time
	updatedAt          time.Time
	deletedAt          *time.Time
}

type NewSubscriptionParams struct {
	ID                 SubscriptionID
	UserID             identityentities.UserID
	Provider           string
	ExternalSubID      valueobjects.ExternalSubscriptionID
	PlanCode           valueobjects.PlanCode
	InitialStatus      valueobjects.SubscriptionStatus
	PeriodStart        time.Time
	PeriodEnd          time.Time
	LastEventAt        time.Time
	LastWebhookEventID valueobjects.WebhookEventID
	CreatedAt          time.Time
}

func NewSubscription(p NewSubscriptionParams) (*Subscription, error) {
	if p.ID.IsZero() {
		return nil, billingdomain.ErrSubscriptionRequiresID
	}
	if p.Provider == "" {
		return nil, billingdomain.ErrSubscriptionRequiresProvider
	}
	if !p.InitialStatus.IsCreatable() {
		return nil, billingdomain.ErrSubscriptionInitialStatusInvalid
	}
	if p.PeriodStart.IsZero() || p.PeriodEnd.IsZero() || !p.PeriodEnd.After(p.PeriodStart) {
		return nil, billingdomain.ErrSubscriptionRequiresPeriod
	}
	period, err := valueobjects.NewBillingPeriodFor(p.PlanCode)
	if err != nil {
		return nil, err
	}
	return &Subscription{
		id:                 p.ID,
		userID:             p.UserID,
		provider:           p.Provider,
		externalSubID:      p.ExternalSubID,
		planCode:           p.PlanCode,
		status:             p.InitialStatus,
		period:             period,
		periodStart:        p.PeriodStart,
		periodEnd:          p.PeriodEnd,
		lastEventAt:        p.LastEventAt,
		lastWebhookEventID: p.LastWebhookEventID,
		createdAt:          p.CreatedAt,
		updatedAt:          p.CreatedAt,
	}, nil
}

func (s *Subscription) ID() SubscriptionID                              { return s.id }
func (s *Subscription) UserID() identityentities.UserID                 { return s.userID }
func (s *Subscription) InternalStatus() valueobjects.SubscriptionStatus { return s.status }
func (s *Subscription) PeriodEnd() time.Time                            { return s.periodEnd }
func (s *Subscription) CurrentPeriodEnd() time.Time                     { return s.periodEnd }
func (s *Subscription) GracePeriodEnd() time.Time                       { return s.gracePeriodEnd }
func (s *Subscription) PlanCode() valueobjects.PlanCode                 { return s.planCode }
func (s *Subscription) ExternalSubscriptionID() valueobjects.ExternalSubscriptionID {
	return s.externalSubID
}
func (s *Subscription) LastEventAt() time.Time                          { return s.lastEventAt }
func (s *Subscription) RefundAmountCents() valueobjects.MoneyBRL        { return s.refundAmountCents }
func (s *Subscription) CreatedAt() time.Time                            { return s.createdAt }
func (s *Subscription) UpdatedAt() time.Time                            { return s.updatedAt }
func (s *Subscription) DeletedAt() *time.Time                           { return s.deletedAt }
func (s *Subscription) LastWebhookEventID() valueobjects.WebhookEventID { return s.lastWebhookEventID }
func (s *Subscription) Provider() string                                { return s.provider }
func (s *Subscription) PeriodStart() time.Time                          { return s.periodStart }

// Status satisfaz identity.domain.services.Subscription por satisfação estrutural (ADR-005).
// Converte o status interno do billing para o enum de identity sem criar dependência inversa.
func (s *Subscription) Status() identityservices.SubscriptionStatus {
	return toIdentityStatus(s.status)
}

// SnapshotForNotification retorna campos relevantes para publicação em events.Bus (best-effort).
func (s *Subscription) SnapshotForNotification() map[string]any {
	return map[string]any{
		"subscription_id":  s.id.String(),
		"user_id":          s.userID.String(),
		"plan_code":        s.planCode.String(),
		"status":           s.status.String(),
		"period_end":       s.periodEnd,
		"grace_period_end": s.gracePeriodEnd,
		"last_event_at":    s.lastEventAt,
	}
}

func (s *Subscription) applyTransition(
	target valueobjects.SubscriptionStatus,
	at time.Time,
	period services.PeriodChange,
) error {
	if err := services.NewStateMachine().AssertLegal(s.status, target); err != nil {
		return err
	}
	s.status = target
	s.lastEventAt = at
	s.updatedAt = at
	if period.AdvancesPeriod() {
		s.periodStart = period.NewStart
		s.periodEnd = period.NewEnd
	}
	if target == valueobjects.SubscriptionStatusPastDue {
		s.gracePeriodEnd = at.Add(services.DefaultGracePeriod)
	}
	return nil
}

func (s *Subscription) Activate(at time.Time, period services.PeriodChange) error {
	return s.applyTransition(valueobjects.SubscriptionStatusActive, at, period)
}

// Renew renova o período de uma assinatura já ativa ou reativa a partir de PastDue.
// Para Active→Active, atualiza apenas o período sem validar transição de estado.
func (s *Subscription) Renew(at time.Time, period services.PeriodChange) error {
	if s.status == valueobjects.SubscriptionStatusActive {
		s.lastEventAt = at
		s.updatedAt = at
		if period.AdvancesPeriod() {
			s.periodStart = period.NewStart
			s.periodEnd = period.NewEnd
		}
		return nil
	}
	return s.applyTransition(valueobjects.SubscriptionStatusActive, at, period)
}

func (s *Subscription) MarkPastDue(at time.Time) error {
	return s.applyTransition(valueobjects.SubscriptionStatusPastDue, at, services.NoPeriodChange())
}

func (s *Subscription) Cancel(at time.Time) error {
	return s.applyTransition(valueobjects.SubscriptionStatusCanceledPending, at, services.NoPeriodChange())
}

func (s *Subscription) Expire(at time.Time) error {
	return s.applyTransition(valueobjects.SubscriptionStatusExpired, at, services.NoPeriodChange())
}

// Refund aplica transição para REFUNDED a partir de qualquer estado não-terminal (RF-17a).
// Política conservadora: chargeback total ou parcial move para REFUNDED imediatamente.
// O amount é armazenado para auditoria; não interfere na decisão de estado.
func (s *Subscription) Refund(at time.Time, amount valueobjects.MoneyBRL, _ valueobjects.TransitionReason) error {
	if err := services.NewStateMachine().AssertLegal(s.status, valueobjects.SubscriptionStatusRefunded); err != nil {
		return err
	}
	s.status = valueobjects.SubscriptionStatusRefunded
	s.refundAmountCents = amount
	s.lastEventAt = at
	s.updatedAt = at
	return nil
}

func toIdentityStatus(s valueobjects.SubscriptionStatus) identityservices.SubscriptionStatus {
	switch s {
	case valueobjects.SubscriptionStatusTrialing:
		return identityservices.StatusTrialing
	case valueobjects.SubscriptionStatusActive:
		return identityservices.StatusActive
	case valueobjects.SubscriptionStatusPastDue:
		return identityservices.StatusPastDue
	case valueobjects.SubscriptionStatusCanceledPending:
		return identityservices.StatusCanceledPending
	case valueobjects.SubscriptionStatusExpired:
		return identityservices.StatusExpired
	case valueobjects.SubscriptionStatusRefunded:
		return identityservices.StatusRefunded
	default:
		return identityservices.StatusUnknown
	}
}
