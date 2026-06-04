package services

import "time"

// Subscription é o contrato mínimo consumido por IsEntitled.
// A implementação concreta vive em internal/billing/domain (Épico E2).
type Subscription interface {
	Status() SubscriptionStatus
	CurrentPeriodEnd() time.Time
	GracePeriodEnd() time.Time
}

// SubscriptionStatus enumera as transições canônicas (iota; zero reservado a Unknown).
type SubscriptionStatus uint8

const (
	StatusUnknown SubscriptionStatus = iota
	StatusTrialing
	StatusActive
	StatusPastDue
	StatusCanceledPending
	StatusExpired
	StatusRefunded
)

func (s SubscriptionStatus) String() string {
	switch s {
	case StatusTrialing:
		return "TRIALING"
	case StatusActive:
		return "ACTIVE"
	case StatusPastDue:
		return "PAST_DUE"
	case StatusCanceledPending:
		return "CANCELED_PENDING"
	case StatusExpired:
		return "EXPIRED"
	case StatusRefunded:
		return "REFUNDED"
	default:
		return "UNKNOWN"
	}
}
