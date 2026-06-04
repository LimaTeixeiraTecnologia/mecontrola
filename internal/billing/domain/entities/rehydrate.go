package entities

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

// RehydrateSubscriptionParams agrupa todos os campos necessários para reconstruir
// uma Subscription a partir de uma row Postgres. Uso restrito ao mapper de infrastructure.
type RehydrateSubscriptionParams struct {
	ID                 SubscriptionID
	UserID             identityentities.UserID
	Provider           string
	ExternalSubID      valueobjects.ExternalSubscriptionID
	PlanCode           valueobjects.PlanCode
	Status             valueobjects.SubscriptionStatus
	Period             valueobjects.BillingPeriod
	PeriodStart        time.Time
	PeriodEnd          time.Time
	GracePeriodEnd     time.Time
	RefundAmountCents  valueobjects.MoneyBRL
	LastEventAt        time.Time
	LastWebhookEventID valueobjects.WebhookEventID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

// RehydrateSubscription é o construtor exclusivo de reidratação (mapper de infrastructure).
// Diferente de NewSubscription, aceita Status e DeletedAt arbitrários — esses já foram
// validados pelo banco via CK constraint. Não publicar para application.
//
// Uso restrito ao mapper de infrastructure.
func RehydrateSubscription(p RehydrateSubscriptionParams) *Subscription {
	return &Subscription{
		id:                 p.ID,
		userID:             p.UserID,
		provider:           p.Provider,
		externalSubID:      p.ExternalSubID,
		planCode:           p.PlanCode,
		status:             p.Status,
		period:             p.Period,
		periodStart:        p.PeriodStart,
		periodEnd:          p.PeriodEnd,
		gracePeriodEnd:     p.GracePeriodEnd,
		refundAmountCents:  p.RefundAmountCents,
		lastEventAt:        p.LastEventAt,
		lastWebhookEventID: p.LastWebhookEventID,
		createdAt:          p.CreatedAt,
		updatedAt:          p.UpdatedAt,
		deletedAt:          p.DeletedAt,
	}
}
