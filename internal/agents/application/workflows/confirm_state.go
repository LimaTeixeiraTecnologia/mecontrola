package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AwaitingKind int

const (
	AwaitingNone AwaitingKind = iota + 1
	AwaitingConfirm
)

func (a AwaitingKind) String() string {
	switch a {
	case AwaitingNone:
		return "none"
	case AwaitingConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a AwaitingKind) IsValid() bool {
	return a >= AwaitingNone && a <= AwaitingConfirm
}

var errInvalidAwaitingKind = errors.New("workflows: awaiting kind inválido")

func ParseAwaitingKind(s string) (AwaitingKind, error) {
	switch s {
	case "none":
		return AwaitingNone, nil
	case "confirm":
		return AwaitingConfirm, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidAwaitingKind, s)
	}
}

type OperationKind int

const (
	OpDeleteEntry OperationKind = iota + 1
	OpEditEntry
	OpDeleteCard
	OpConfirmRegister
	OpUpdateRecurrence
	OpDeleteRecurrence
	OpUpdateCard
)

func (o OperationKind) String() string {
	switch o {
	case OpDeleteEntry:
		return "delete_entry"
	case OpEditEntry:
		return "edit_entry"
	case OpDeleteCard:
		return "delete_card"
	case OpConfirmRegister:
		return "confirm_register"
	case OpUpdateRecurrence:
		return "update_recurrence"
	case OpDeleteRecurrence:
		return "delete_recurrence"
	case OpUpdateCard:
		return "update_card"
	default:
		return "unknown"
	}
}

func (o OperationKind) IsValid() bool {
	return o >= OpDeleteEntry && o <= OpUpdateCard
}

var errInvalidOperationKind = errors.New("workflows: operation kind inválido")

func ParseOperationKind(s string) (OperationKind, error) {
	switch s {
	case "delete_entry":
		return OpDeleteEntry, nil
	case "edit_entry":
		return OpEditEntry, nil
	case "delete_card":
		return OpDeleteCard, nil
	case "confirm_register":
		return OpConfirmRegister, nil
	case "update_recurrence":
		return OpUpdateRecurrence, nil
	case "delete_recurrence":
		return OpDeleteRecurrence, nil
	case "update_card":
		return OpUpdateCard, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidOperationKind, s)
	}
}

type ConfirmState struct {
	Awaiting      AwaitingKind  `json:"awaiting"`
	Operation     OperationKind `json:"operation"`
	TargetRef     string        `json:"targetRef"`
	TargetKind    string        `json:"targetKind"`
	ImpactNote    string        `json:"impactNote"`
	RepromptDone  bool          `json:"repromptDone"`
	MessageID     string        `json:"messageId"`
	SuspendedAt   time.Time     `json:"suspendedAt"`
	ResumeText    string        `json:"resumeText"`
	UpdatePayload string        `json:"updatePayload"`
	ResponseText  string        `json:"responseText"`
	UserID        uuid.UUID     `json:"userId"`
	Version       int64         `json:"version"`
}
