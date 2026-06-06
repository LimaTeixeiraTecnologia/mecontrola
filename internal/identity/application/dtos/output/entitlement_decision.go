package output

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
)

type EntitlementDecision struct {
	Entitled     bool
	Reason       string
	Subscription entitlementSubscription
}

type entitlementSubscription struct {
	SubscriptionID string
	Status         string
	PeriodEnd      time.Time
	GraceEnd       time.Time
}

func NewEntitlementDecision(entitled bool, reason domain.Reason, subscriptionID string, status string, periodEnd time.Time, graceEnd time.Time) EntitlementDecision {
	return EntitlementDecision{
		Entitled: entitled,
		Reason:   string(reason),
		Subscription: entitlementSubscription{
			SubscriptionID: subscriptionID,
			Status:         status,
			PeriodEnd:      periodEnd,
			GraceEnd:       graceEnd,
		},
	}
}
