package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BudgetAwaitingSlot int

const (
	AwaitingBudgetTotal BudgetAwaitingSlot = iota + 1
	AwaitingBudgetDistribution
	AwaitingBudgetConfirm
)

func (a BudgetAwaitingSlot) String() string {
	switch a {
	case AwaitingBudgetTotal:
		return "total"
	case AwaitingBudgetDistribution:
		return "distribution"
	case AwaitingBudgetConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a BudgetAwaitingSlot) IsValid() bool {
	return a >= AwaitingBudgetTotal && a <= AwaitingBudgetConfirm
}

var errInvalidBudgetAwaitingSlot = errors.New("workflows: budget awaiting slot inválido")

func ParseBudgetAwaitingSlot(s string) (BudgetAwaitingSlot, error) {
	switch s {
	case "total":
		return AwaitingBudgetTotal, nil
	case "distribution":
		return AwaitingBudgetDistribution, nil
	case "confirm":
		return AwaitingBudgetConfirm, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidBudgetAwaitingSlot, s)
	}
}

type BudgetCreationStatus int

const (
	BudgetCreationActive BudgetCreationStatus = iota + 1
	BudgetCreationCompleted
	BudgetCreationCancelled
	BudgetCreationExpired
)

func (b BudgetCreationStatus) String() string {
	switch b {
	case BudgetCreationActive:
		return "active"
	case BudgetCreationCompleted:
		return "completed"
	case BudgetCreationCancelled:
		return "cancelled"
	case BudgetCreationExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (b BudgetCreationStatus) IsValid() bool {
	return b >= BudgetCreationActive && b <= BudgetCreationExpired
}

var errInvalidBudgetCreationStatus = errors.New("workflows: budget creation status inválido")

func ParseBudgetCreationStatus(s string) (BudgetCreationStatus, error) {
	switch s {
	case "active":
		return BudgetCreationActive, nil
	case "completed":
		return BudgetCreationCompleted, nil
	case "cancelled":
		return BudgetCreationCancelled, nil
	case "expired":
		return BudgetCreationExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidBudgetCreationStatus, s)
	}
}

type BudgetCreationState struct {
	Status            BudgetCreationStatus `json:"status"`
	Awaiting          BudgetAwaitingSlot   `json:"awaiting"`
	UserID            uuid.UUID            `json:"userId"`
	Competence        string               `json:"competence"`
	TotalCents        int64                `json:"totalCents"`
	Allocations       map[string]int       `json:"allocations"`
	ResumeText        string               `json:"resumeText"`
	ResponseText      string               `json:"responseText"`
	RepromptCount     int                  `json:"repromptCount"`
	MessageID         string               `json:"messageId"`
	IncomingMessageID string               `json:"incomingMessageId"`
	SuspendedAt       time.Time            `json:"suspendedAt"`
	Expired           bool                 `json:"expired"`
}
