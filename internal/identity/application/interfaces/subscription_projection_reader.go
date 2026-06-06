package interfaces

import (
	"context"
	"time"
)

type SubscriptionProjectionRecord struct {
	SubscriptionID string    `json:"subscription_id"`
	FunnelToken    string    `json:"funnel_token"`
	Status         string    `json:"status"`
	PeriodEnd      time.Time `json:"period_end"`
	GraceEnd       time.Time `json:"grace_end,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
	UserID         string    `json:"-"`
}

type SubscriptionProjectionReader interface {
	FindCurrentBySubscriptionID(ctx context.Context, subscriptionID string) (SubscriptionProjectionRecord, error)
}
