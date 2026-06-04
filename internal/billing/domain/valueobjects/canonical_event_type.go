package valueobjects

type CanonicalEventType uint8

const (
	CanonicalEventUnknown CanonicalEventType = iota
	CanonicalEventPurchaseApproved
	CanonicalEventRenewed
	CanonicalEventLate
	CanonicalEventCanceled
	CanonicalEventRefunded
	CanonicalEventChargeback
	CanonicalEventExpired
)

func (c CanonicalEventType) String() string {
	switch c {
	case CanonicalEventPurchaseApproved:
		return "purchase_approved"
	case CanonicalEventRenewed:
		return "renewed"
	case CanonicalEventLate:
		return "late"
	case CanonicalEventCanceled:
		return "canceled"
	case CanonicalEventRefunded:
		return "refunded"
	case CanonicalEventChargeback:
		return "chargeback"
	case CanonicalEventExpired:
		return "expired"
	default:
		return "UNKNOWN"
	}
}
