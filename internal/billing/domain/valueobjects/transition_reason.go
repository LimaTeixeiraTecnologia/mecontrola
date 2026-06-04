package valueobjects

type TransitionReason uint8

const (
	TransitionReasonUnknown TransitionReason = iota
	TransitionReasonPurchaseApproved
	TransitionReasonRenewed
	TransitionReasonLate
	TransitionReasonCanceled
	TransitionReasonRefunded
	TransitionReasonChargebackReceived
	TransitionReasonReconciliationSync
)

func (r TransitionReason) String() string {
	switch r {
	case TransitionReasonPurchaseApproved:
		return "purchase_approved"
	case TransitionReasonRenewed:
		return "renewed"
	case TransitionReasonLate:
		return "late"
	case TransitionReasonCanceled:
		return "canceled"
	case TransitionReasonRefunded:
		return "refunded"
	case TransitionReasonChargebackReceived:
		return "chargeback_received"
	case TransitionReasonReconciliationSync:
		return "reconciliation_sync"
	default:
		return "UNKNOWN"
	}
}
