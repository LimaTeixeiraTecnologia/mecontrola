package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DestructiveOperationKind int

const (
	DestructiveOpDeleteCard DestructiveOperationKind = iota + 1
	DestructiveOpDeleteRecurrence
	DestructiveOpDeleteEntry
	DestructiveOpUpdateRecurrence
)

func (o DestructiveOperationKind) String() string {
	switch o {
	case DestructiveOpDeleteCard:
		return "delete_card"
	case DestructiveOpDeleteRecurrence:
		return "delete_recurrence"
	case DestructiveOpDeleteEntry:
		return "delete_entry"
	case DestructiveOpUpdateRecurrence:
		return "update_recurrence"
	default:
		return "unknown"
	}
}

func (o DestructiveOperationKind) IsValid() bool {
	return o >= DestructiveOpDeleteCard && o <= DestructiveOpUpdateRecurrence
}

var errInvalidDestructiveOperationKind = errors.New("workflows: destructive operation kind inválido")

func ParseDestructiveOperationKind(s string) (DestructiveOperationKind, error) {
	switch s {
	case "delete_card":
		return DestructiveOpDeleteCard, nil
	case "delete_recurrence":
		return DestructiveOpDeleteRecurrence, nil
	case "delete_entry":
		return DestructiveOpDeleteEntry, nil
	case "update_recurrence":
		return DestructiveOpUpdateRecurrence, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidDestructiveOperationKind, s)
	}
}

type DestructiveManageStatus int

const (
	DestructiveManageActive DestructiveManageStatus = iota + 1
	DestructiveManageCompleted
	DestructiveManageCancelled
	DestructiveManageExpired
)

func (s DestructiveManageStatus) String() string {
	switch s {
	case DestructiveManageActive:
		return "active"
	case DestructiveManageCompleted:
		return "completed"
	case DestructiveManageCancelled:
		return "cancelled"
	case DestructiveManageExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (s DestructiveManageStatus) IsValid() bool {
	return s >= DestructiveManageActive && s <= DestructiveManageExpired
}

var errInvalidDestructiveManageStatus = errors.New("workflows: destructive manage status inválido")

func ParseDestructiveManageStatus(s string) (DestructiveManageStatus, error) {
	switch s {
	case "active":
		return DestructiveManageActive, nil
	case "completed":
		return DestructiveManageCompleted, nil
	case "cancelled":
		return DestructiveManageCancelled, nil
	case "expired":
		return DestructiveManageExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidDestructiveManageStatus, s)
	}
}

type DestructiveManageState struct {
	Status            DestructiveManageStatus  `json:"status"`
	Operation         DestructiveOperationKind `json:"operation"`
	UserID            uuid.UUID                `json:"userId"`
	TargetRef         string                   `json:"targetRef"`
	ImpactNote        string                   `json:"impactNote"`
	RepromptDone      bool                     `json:"repromptDone"`
	MessageID         string                   `json:"messageId"`
	IncomingMessageID string                   `json:"incomingMessageId"`
	SuspendedAt       time.Time                `json:"suspendedAt"`
	ResumeText        string                   `json:"resumeText"`
	ResponseText      string                   `json:"responseText"`
	Version           int64                    `json:"version"`
	Expired           bool                     `json:"expired"`
	UpdatePayload     string                   `json:"updatePayload"`
}
