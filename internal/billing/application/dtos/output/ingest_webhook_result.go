package output

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"

// IngestWebhookResult é o resultado do caso de uso de ingestão de webhook.
type IngestWebhookResult struct {
	Duplicate      bool
	WebhookEventID valueobjects.WebhookEventID
}
