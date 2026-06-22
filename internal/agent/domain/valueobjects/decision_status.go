package valueobjects

import (
	"errors"
	"strings"
)

var ErrDecisionStatusInvalid = errors.New("agent.decision: status inválido")

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

func ParseDecisionStatus(raw string) (DecisionStatus, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case decisionStatusPendingValue:
		return DecisionStatusPending, nil
	case decisionStatusExecutedValue:
		return DecisionStatusExecuted, nil
	case decisionStatusRejectedValue:
		return DecisionStatusRejected, nil
	case decisionStatusAwaitingConfirmationValue:
		return DecisionStatusAwaitingConfirmation, nil
	default:
		return 0, ErrDecisionStatusInvalid
	}
}
