package input

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"

// ProcessEventInput identifica o evento de outbox a ser processado pelo BillingEventProcessor.
type ProcessEventInput struct {
	WebhookEventID valueobjects.WebhookEventID
}
