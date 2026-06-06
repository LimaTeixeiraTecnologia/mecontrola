package output

import "time"

type EntitlementDecisionInput struct {
	UserID         string
	SubscriptionID string
	Status         string
	PeriodEnd      time.Time
	GraceEnd       time.Time
}
