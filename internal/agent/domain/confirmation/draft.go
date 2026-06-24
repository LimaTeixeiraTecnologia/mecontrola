package confirmation

import (
	"errors"
	"fmt"
	"time"
)

type OperationKind int

const (
	OperationDeleteLast OperationKind = iota + 1
	OperationEditLast
	OperationDeleteCard
	OperationBudgetCommit
)

func (o OperationKind) String() string {
	switch o {
	case OperationDeleteLast:
		return "delete_last"
	case OperationEditLast:
		return "edit_last"
	case OperationDeleteCard:
		return "delete_card"
	case OperationBudgetCommit:
		return "budget_commit"
	default:
		return "unknown"
	}
}

func (o OperationKind) IsValid() bool {
	return o >= OperationDeleteLast && o <= OperationBudgetCommit
}

var errInvalidOperationKind = errors.New("confirmation: invalid operation kind")

func ParseOperationKind(s string) (OperationKind, error) {
	switch s {
	case "delete_last":
		return OperationDeleteLast, nil
	case "edit_last":
		return OperationEditLast, nil
	case "delete_card":
		return OperationDeleteCard, nil
	case "budget_commit":
		return OperationBudgetCommit, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidOperationKind, s)
	}
}

type AwaitingApproval int

const (
	AwaitingNone AwaitingApproval = iota
	AwaitingConfirm
)

func (a AwaitingApproval) String() string {
	switch a {
	case AwaitingNone:
		return "none"
	case AwaitingConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a AwaitingApproval) IsValid() bool {
	return a >= AwaitingNone && a <= AwaitingConfirm
}

var errInvalidAwaitingApproval = errors.New("confirmation: invalid awaiting approval")

func ParseAwaitingApproval(s string) (AwaitingApproval, error) {
	switch s {
	case "none":
		return AwaitingNone, nil
	case "confirm":
		return AwaitingConfirm, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidAwaitingApproval, s)
	}
}

type ConfirmState struct {
	OperationKind    OperationKind    `json:"operation_kind"`
	AwaitingApproval AwaitingApproval `json:"awaiting_approval"`
	RepromptCount    int              `json:"reprompt_count"`
	MessageID        string           `json:"message_id"`
	SuspendedAt      time.Time        `json:"suspended_at"`
	ShortCircuit     bool             `json:"short_circuit"`
	Expired          bool             `json:"expired"`
	ResumeText       string           `json:"resume_text"`
	UserID           string           `json:"user_id"`
	Channel          string           `json:"channel"`
	PromptText       string           `json:"prompt_text"`
	Reply            string           `json:"reply"`
	Outcome          int              `json:"outcome"`
}

func (s ConfirmState) IsDone() bool {
	return s.ShortCircuit
}
