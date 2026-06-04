package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type CanonicalSubscription struct {
	ExternalID  string
	Status      valueobjects.SubscriptionStatus
	PlanCode    valueobjects.PlanCode
	PeriodStart time.Time
	PeriodEnd   time.Time
	Customer    CanonicalCustomer
}
