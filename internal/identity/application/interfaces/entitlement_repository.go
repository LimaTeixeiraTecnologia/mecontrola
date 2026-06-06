package interfaces

import (
	"context"
	"errors"
	"time"
)

var ErrEntitlementNotFound = errors.New("identity: entitlement not found")

type EntitlementRecord struct {
	UserID         string
	SubscriptionID string
	Status         string
	PeriodEnd      time.Time
	GraceEnd       time.Time
}

type EntitlementRepository interface {
	Upsert(ctx context.Context, record EntitlementRecord) error
	FindByUserID(ctx context.Context, userID string) (EntitlementRecord, error)
	UpsertPending(ctx context.Context, subscriptionID string, funnelToken string, payload []byte) error
}

type EntitlementReader interface {
	FindByUserID(ctx context.Context, userID string) (EntitlementRecord, error)
}
