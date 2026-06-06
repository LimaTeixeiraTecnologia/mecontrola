package consumers

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type sendSubscriptionNotificationUseCase interface {
	Execute(ctx context.Context, in input.SendSubscriptionNotificationInput) error
}

type NotificationHandler struct {
	usecase   sendSubscriptionNotificationUseCase
	eventType string
	o11y      observability.Observability
}

func NewNotificationHandler(
	uc sendSubscriptionNotificationUseCase,
	eventType string,
	o11y observability.Observability,
) *NotificationHandler {
	return &NotificationHandler{
		usecase:   uc,
		eventType: eventType,
		o11y:      o11y,
	}
}

func (h *NotificationHandler) Handle(ctx context.Context, event events.Event) error {
	ctx, span := h.o11y.Tracer().Start(ctx, "billing.handler.notification.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return nil
	}

	return h.usecase.Execute(ctx, input.SendSubscriptionNotificationInput{
		EventType: h.eventType,
		Payload:   env.Payload,
	})
}
