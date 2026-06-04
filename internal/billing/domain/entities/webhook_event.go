package entities

import (
	"encoding/json"
	"time"

	billingdomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

// WebhookEvent é a projeção imutável da row webhook_events. Value type — não é aggregate root.
type WebhookEvent struct {
	id              valueobjects.WebhookEventID
	provider        string
	externalEventID valueobjects.ExternalEventID
	eventType       string
	signature       string
	headersJSON     json.RawMessage
	payload         json.RawMessage
	receivedAt      time.Time
}

type NewWebhookEventParams struct {
	ID              valueobjects.WebhookEventID
	Provider        string
	ExternalEventID valueobjects.ExternalEventID
	EventType       string
	Signature       string
	HeadersJSON     json.RawMessage
	Payload         json.RawMessage
	ReceivedAt      time.Time
}

func NewWebhookEvent(p NewWebhookEventParams) (WebhookEvent, error) {
	if p.ID.IsZero() {
		return WebhookEvent{}, billingdomain.ErrWebhookEventRequiresID
	}
	if p.Provider == "" {
		return WebhookEvent{}, billingdomain.ErrWebhookEventRequiresProvider
	}
	if p.ExternalEventID.IsZero() {
		return WebhookEvent{}, billingdomain.ErrWebhookEventRequiresExternalID
	}
	if len(p.Payload) == 0 {
		return WebhookEvent{}, billingdomain.ErrWebhookEventRequiresPayload
	}
	return WebhookEvent{
		id:              p.ID,
		provider:        p.Provider,
		externalEventID: p.ExternalEventID,
		eventType:       p.EventType,
		signature:       p.Signature,
		headersJSON:     p.HeadersJSON,
		payload:         p.Payload,
		receivedAt:      p.ReceivedAt,
	}, nil
}

func (w WebhookEvent) ID() valueobjects.WebhookEventID               { return w.id }
func (w WebhookEvent) Provider() string                              { return w.provider }
func (w WebhookEvent) ExternalEventID() valueobjects.ExternalEventID { return w.externalEventID }
func (w WebhookEvent) EventType() string                             { return w.eventType }
func (w WebhookEvent) Signature() string                             { return w.signature }
func (w WebhookEvent) HeadersJSON() json.RawMessage                  { return w.headersJSON }
func (w WebhookEvent) Payload() json.RawMessage                      { return w.payload }
func (w WebhookEvent) ReceivedAt() time.Time                         { return w.receivedAt }
