package outbox

import (
	"encoding/json"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

// ReceivedPayload é o pointer mínimo publicado no outbox para billing.kiwify.received (ADR-001).
// Contém apenas referência ao webhook_event_id para que o processor busque o payload bruto.
type ReceivedPayload struct {
	WebhookEventID string `json:"webhook_event_id"`
	Provider       string `json:"provider"`
}

// EncodeReceivedPayload serializa um ReceivedPayload para JSON para uso no outbox.
func EncodeReceivedPayload(id valueobjects.WebhookEventID, provider string) (json.RawMessage, error) {
	raw, err := json.Marshal(ReceivedPayload{
		WebhookEventID: id.String(),
		Provider:       provider,
	})
	if err != nil {
		return nil, fmt.Errorf("outbox: encode received payload: %w", err)
	}
	return raw, nil
}

// DecodeReceivedPayload desserializa um ReceivedPayload do JSON bruto do outbox.
func DecodeReceivedPayload(raw json.RawMessage) (ReceivedPayload, error) {
	var payload ReceivedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ReceivedPayload{}, fmt.Errorf("outbox: decode received payload: %w", err)
	}
	return payload, nil
}
