package valueobjects

type DecisionStatus int

const (
	DecisionStatusPending DecisionStatus = iota + 1
	DecisionStatusExecuted
	DecisionStatusRejected
	DecisionStatusAwaitingConfirmation
)

const (
	decisionStatusPendingValue              = "pending"
	decisionStatusExecutedValue             = "executed"
	decisionStatusRejectedValue             = "rejected"
	decisionStatusAwaitingConfirmationValue = "awaiting_confirmation"
)

func (s DecisionStatus) String() string {
	switch s {
	case DecisionStatusPending:
		return decisionStatusPendingValue
	case DecisionStatusExecuted:
		return decisionStatusExecutedValue
	case DecisionStatusRejected:
		return decisionStatusRejectedValue
	case DecisionStatusAwaitingConfirmation:
		return decisionStatusAwaitingConfirmationValue
	default:
		return ""
	}
}

func (s DecisionStatus) IsZero() bool { return s == 0 }

func (s DecisionStatus) IsSettled() bool {
	return s == DecisionStatusExecuted || s == DecisionStatusRejected
}
