package interfaces

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

// WebhookEventRepository é o port de persistência do event store imutável webhook_events
// e da tabela de deduplicação billing_event_applications.
type WebhookEventRepository interface {
	// InsertIfNew persiste o evento via INSERT ... ON CONFLICT DO NOTHING.
	// Retorna (false, nil) em duplicata — não é erro.
	InsertIfNew(ctx context.Context, event entities.WebhookEvent) (inserted bool, err error)
	FindRawPayload(ctx context.Context, id valueobjects.WebhookEventID) (json.RawMessage, error)
	MarkProcessed(ctx context.Context, id valueobjects.WebhookEventID, at time.Time) error
	// RecordApplication registra a aplicação de um evento em billing_event_applications.
	// Retorna (false, nil) em conflict (evento já aplicado) — idempotência do processor (ADR-009).
	RecordApplication(ctx context.Context, eventID valueobjects.WebhookEventID, subID entities.SubscriptionID, at time.Time) (recorded bool, err error)
	ListPendingAnonymization(ctx context.Context, olderThan time.Time, limit int) ([]entities.WebhookEvent, error)
	Anonymize(ctx context.Context, id valueobjects.WebhookEventID, redacted json.RawMessage, at time.Time) error
}
