package outbox_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	billingoutbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	platformoutbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type HandlerRegistrationSuite struct {
	suite.Suite
}

func TestHandlerRegistration(t *testing.T) {
	suite.Run(t, new(HandlerRegistrationSuite))
}

func (s *HandlerRegistrationSuite) TestRegisterHandlersRegistersWebhookAndReconciliationEvents() {
	registry := platformoutbox.NewRegistry()

	err := billingoutbox.RegisterHandlers(registry, &usecases.ProcessBillingEventUseCase{})

	s.Require().NoError(err)
	webhookEvent, _ := events.NewEventName("billing.kiwify.received")
	reconciliationEvent, _ := events.NewEventName("billing.reconciliation.divergence_detected")
	s.Len(registry.SubscriptionsFor(webhookEvent), 1)
	s.Len(registry.SubscriptionsFor(reconciliationEvent), 1)
}
