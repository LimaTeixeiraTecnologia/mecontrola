package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type rowMapper struct{}

type subscriptionRow struct {
	ID                     string
	UserID                 string
	Provider               string
	ExternalSubscriptionID string
	PlanCode               string
	Status                 string
	PeriodStart            time.Time
	PeriodEnd              time.Time
	GracePeriodEnd         time.Time
	RefundAmountCents      int64
	LastEventAt            time.Time
	LastWebhookEventID     string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              *time.Time
}

type webhookEventRow struct {
	ID              string
	Provider        string
	ExternalEventID string
	EventType       string
	Signature       string
	HeadersJSON     []byte
	Payload         []byte
	ReceivedAt      time.Time
}

func (m rowMapper) hydrateSubscription(r subscriptionRow) (*entities.Subscription, error) {
	subID, err := entities.NewSubscriptionID(r.ID)
	if err != nil {
		return nil, fmt.Errorf("mapper: subscription id: %w", err)
	}

	userID, err := identityentities.NewUserID(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("mapper: user id: %w", err)
	}

	planCode, err := valueobjects.ParsePlanCode(r.PlanCode)
	if err != nil {
		return nil, fmt.Errorf("mapper: plan code %q: %w", r.PlanCode, err)
	}

	status, err := m.parseSubscriptionStatus(r.Status)
	if err != nil {
		return nil, fmt.Errorf("mapper: status %q: %w", r.Status, err)
	}

	period, err := valueobjects.NewBillingPeriodFor(planCode)
	if err != nil {
		return nil, fmt.Errorf("mapper: billing period: %w", err)
	}

	externalSubID, err := valueobjects.NewExternalSubscriptionID(r.ExternalSubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("mapper: external subscription id: %w", err)
	}

	lastWebhookEventID, err := valueobjects.NewWebhookEventID(r.LastWebhookEventID)
	if err != nil {
		return nil, fmt.Errorf("mapper: last webhook event id: %w", err)
	}

	refundAmount, err := valueobjects.NewMoneyBRL(r.RefundAmountCents)
	if err != nil {
		return nil, fmt.Errorf("mapper: refund amount: %w", err)
	}

	return entities.RehydrateSubscription(entities.RehydrateSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           r.Provider,
		ExternalSubID:      externalSubID,
		PlanCode:           planCode,
		Status:             status,
		Period:             period,
		PeriodStart:        r.PeriodStart,
		PeriodEnd:          r.PeriodEnd,
		GracePeriodEnd:     r.GracePeriodEnd,
		RefundAmountCents:  refundAmount,
		LastEventAt:        r.LastEventAt,
		LastWebhookEventID: lastWebhookEventID,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
		DeletedAt:          r.DeletedAt,
	}), nil
}

func (m rowMapper) hydrateWebhookEvent(r webhookEventRow) (entities.WebhookEvent, error) {
	id, err := valueobjects.NewWebhookEventID(r.ID)
	if err != nil {
		return entities.WebhookEvent{}, fmt.Errorf("mapper: webhook event id: %w", err)
	}

	externalEventID, err := valueobjects.NewExternalEventIDCascade([]byte(`{"id":"` + r.ExternalEventID + `"}`))
	if err != nil {
		return entities.WebhookEvent{}, fmt.Errorf("mapper: external event id: %w", err)
	}

	headersJSON := json.RawMessage(r.HeadersJSON)
	if len(headersJSON) == 0 {
		headersJSON = json.RawMessage("{}")
	}

	return entities.NewWebhookEvent(entities.NewWebhookEventParams{
		ID:              id,
		Provider:        r.Provider,
		ExternalEventID: externalEventID,
		EventType:       r.EventType,
		Signature:       r.Signature,
		HeadersJSON:     headersJSON,
		Payload:         json.RawMessage(r.Payload),
		ReceivedAt:      r.ReceivedAt,
	})
}

func (m rowMapper) parseSubscriptionStatus(s string) (valueobjects.SubscriptionStatus, error) {
	switch s {
	case "TRIALING":
		return valueobjects.SubscriptionStatusTrialing, nil
	case "ACTIVE":
		return valueobjects.SubscriptionStatusActive, nil
	case "PAST_DUE":
		return valueobjects.SubscriptionStatusPastDue, nil
	case "CANCELED_PENDING":
		return valueobjects.SubscriptionStatusCanceledPending, nil
	case "EXPIRED":
		return valueobjects.SubscriptionStatusExpired, nil
	case "REFUNDED":
		return valueobjects.SubscriptionStatusRefunded, nil
	default:
		return valueobjects.SubscriptionStatusUnknown, fmt.Errorf("status desconhecido: %q", s)
	}
}
