package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type notificationPayloadBase struct {
	SubscriptionID string `json:"subscription_id"`
}

type NotificationHandler struct {
	sender    interfaces.NotificationSender
	eventType string
	o11y      observability.Observability
	failures  observability.Counter
}

func NewNotificationHandler(
	sender interfaces.NotificationSender,
	eventType string,
	o11y observability.Observability,
) *NotificationHandler {
	failures := o11y.Metrics().Counter(
		"billing_notification_failures_total",
		"Total de falhas de envio de notificação por trigger",
		"1",
	)
	return &NotificationHandler{
		sender:    sender,
		eventType: eventType,
		o11y:      o11y,
		failures:  failures,
	}
}

func (h *NotificationHandler) Handle(ctx context.Context, event events.Event) error {
	ctx, span := h.o11y.Tracer().Start(ctx, "billing.handler.notification.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return nil
	}

	var base notificationPayloadBase
	if err := json.Unmarshal(env.Payload, &base); err != nil {
		h.o11y.Logger().Error(ctx, "billing.notification.failed",
			observability.String("event_type", h.eventType),
			observability.Error(fmt.Errorf("unmarshal: %w", err)),
		)
		h.failures.Add(ctx, 1, observability.String("trigger", h.eventType))
		return nil
	}

	payload := interfaces.NotificationPayload{
		SubscriptionID: base.SubscriptionID,
		EventType:      h.eventType,
	}

	if err := h.sender.NotifyTransition(ctx, payload); err != nil {
		h.o11y.Logger().Error(ctx, "billing.notification.failed",
			observability.String("subscription_id", base.SubscriptionID),
			observability.String("trigger", h.eventType),
			observability.Error(err),
		)
		h.failures.Add(ctx, 1, observability.String("trigger", h.eventType))
	}

	return nil
}
