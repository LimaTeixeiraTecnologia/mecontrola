package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type notificationPayloadBase struct {
	SubscriptionID string `json:"subscription_id"`
}

type SendSubscriptionNotification struct {
	sender   interfaces.NotificationSender
	o11y     observability.Observability
	failures observability.Counter
}

func NewSendSubscriptionNotification(
	sender interfaces.NotificationSender,
	o11y observability.Observability,
) *SendSubscriptionNotification {
	failures := o11y.Metrics().Counter(
		"billing_notification_failures_total",
		"Total de falhas de envio de notificação por trigger",
		"1",
	)
	return &SendSubscriptionNotification{sender: sender, o11y: o11y, failures: failures}
}

func (u *SendSubscriptionNotification) Execute(ctx context.Context, in input.SendSubscriptionNotificationInput) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "billing.usecase.send_subscription_notification")
	defer span.End()

	if err := in.Validate(); err != nil {
		return err
	}

	var base notificationPayloadBase
	if err := json.Unmarshal(in.Payload, &base); err != nil {
		u.o11y.Logger().Error(ctx, "billing.notification.failed",
			observability.String("event_type", in.EventType),
			observability.Error(fmt.Errorf("unmarshal: %w", err)),
		)
		u.failures.Add(ctx, 1, observability.String("trigger", in.EventType))
		return nil
	}

	if err := u.sender.NotifyTransition(ctx, interfaces.NotificationPayload{
		SubscriptionID: base.SubscriptionID,
		EventType:      in.EventType,
	}); err != nil {
		u.o11y.Logger().Error(ctx, "billing.notification.failed",
			observability.String("subscription_id", base.SubscriptionID),
			observability.String("trigger", in.EventType),
			observability.Error(err),
		)
		u.failures.Add(ctx, 1, observability.String("trigger", in.EventType))
	}

	return nil
}
