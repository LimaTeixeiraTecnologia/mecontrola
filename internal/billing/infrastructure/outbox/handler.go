package outbox

import (
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	platformoutbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	billingEventProcessorName          = "billing-event-processor"
	billingEventProcessorType          = "billing.kiwify.received"
	billingReconciliationProcessorType = "billing.reconciliation.divergence_detected"
)

// RegisterHandlers registra o BillingEventProcessor no outbox.Registry.
// DEVE ser chamado antes do Dispatcher.Start para que eventos não sejam enviados à DLQ (RF-47).
func RegisterHandlers(registry platformoutbox.Registry, processor *usecases.ProcessBillingEventUseCase) error {
	name, err := platformoutbox.NewSubscriptionName(billingEventProcessorName)
	if err != nil {
		return fmt.Errorf("billing outbox: subscription name: %w", err)
	}

	for _, rawType := range []string{billingEventProcessorType, billingReconciliationProcessorType} {
		eventType, err := events.NewEventName(rawType)
		if err != nil {
			return fmt.Errorf("billing outbox: event type: %w", err)
		}
		if err := registry.Register(platformoutbox.Subscription{
			Name:      name,
			EventType: eventType,
			Handler:   processor.Handle,
		}); err != nil {
			return fmt.Errorf("billing outbox: registro handler: %w", err)
		}
	}

	return nil
}
