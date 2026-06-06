package interfaces

import "context"

type NotificationPayload struct {
	SubscriptionID string
	EventType      string
	UserID         string
	FunnelToken    string
}

type NotificationSender interface {
	NotifyTransition(ctx context.Context, payload NotificationPayload) error
}
