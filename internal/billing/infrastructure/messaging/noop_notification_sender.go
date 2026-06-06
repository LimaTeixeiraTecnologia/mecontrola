package messaging

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type NoopNotificationSender struct{}

func NewNoopNotificationSender() interfaces.NotificationSender {
	return &NoopNotificationSender{}
}

func (n *NoopNotificationSender) NotifyTransition(_ context.Context, _ interfaces.NotificationPayload) error {
	return nil
}
