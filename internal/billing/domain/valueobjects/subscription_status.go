package valueobjects

type SubscriptionStatus uint8

const (
	SubscriptionStatusUnknown SubscriptionStatus = iota
	SubscriptionStatusTrialing
	SubscriptionStatusActive
	SubscriptionStatusPastDue
	SubscriptionStatusCanceledPending
	SubscriptionStatusExpired
	SubscriptionStatusRefunded
)

func (s SubscriptionStatus) String() string {
	switch s {
	case SubscriptionStatusTrialing:
		return "TRIALING"
	case SubscriptionStatusActive:
		return "ACTIVE"
	case SubscriptionStatusPastDue:
		return "PAST_DUE"
	case SubscriptionStatusCanceledPending:
		return "CANCELED_PENDING"
	case SubscriptionStatusExpired:
		return "EXPIRED"
	case SubscriptionStatusRefunded:
		return "REFUNDED"
	default:
		return "UNKNOWN"
	}
}

func (s SubscriptionStatus) IsCreatable() bool {
	switch s {
	case SubscriptionStatusActive, SubscriptionStatusTrialing:
		return true
	default:
		return false
	}
}
