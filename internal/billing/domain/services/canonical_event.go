package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type CanonicalCustomer struct {
	WhatsApp identityvo.WhatsAppNumber
	Email    string
}

type CanonicalEvent struct {
	Type                   valueobjects.CanonicalEventType
	ExternalEventID        string
	ExternalSubscriptionID string
	PlanCode               valueobjects.PlanCode
	OccurredAt             time.Time
	PeriodStart            time.Time
	PeriodEnd              time.Time
	SignupToken            string
	Customer               CanonicalCustomer
	RefundAmountCents      int64
}
