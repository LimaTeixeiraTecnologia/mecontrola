package domain

import "errors"

var (
	ErrInvalidSubscriptionID            = errors.New("billing: subscription id inválido")
	ErrSubscriptionRequiresID           = errors.New("billing: subscription requer id")
	ErrSubscriptionRequiresProvider     = errors.New("billing: subscription requer provider")
	ErrSubscriptionInitialStatusInvalid = errors.New("billing: status inicial não permitido na criação")
	ErrSubscriptionRequiresPeriod       = errors.New("billing: subscription requer period_start < period_end")
	ErrWebhookEventRequiresID           = errors.New("billing: webhook event requer id")
	ErrWebhookEventRequiresProvider     = errors.New("billing: webhook event requer provider")
	ErrWebhookEventRequiresExternalID   = errors.New("billing: webhook event requer external_event_id")
	ErrWebhookEventRequiresPayload      = errors.New("billing: webhook event requer payload")
)
