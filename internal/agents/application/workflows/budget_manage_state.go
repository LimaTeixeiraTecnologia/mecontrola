package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BudgetManageOperationKind int

const (
	BudgetManageOpCreateRetroactive BudgetManageOperationKind = iota + 1
	BudgetManageOpEditTotal
	BudgetManageOpEditDistribution
)

func (o BudgetManageOperationKind) String() string {
	switch o {
	case BudgetManageOpCreateRetroactive:
		return "create_retroactive"
	case BudgetManageOpEditTotal:
		return "edit_total"
	case BudgetManageOpEditDistribution:
		return "edit_distribution"
	default:
		return "unknown"
	}
}

func (o BudgetManageOperationKind) IsValid() bool {
	return o >= BudgetManageOpCreateRetroactive && o <= BudgetManageOpEditDistribution
}

var errInvalidBudgetManageOperationKind = errors.New("workflows: budget manage operation kind inválido")

func ParseBudgetManageOperationKind(s string) (BudgetManageOperationKind, error) {
	switch s {
	case "create_retroactive":
		return BudgetManageOpCreateRetroactive, nil
	case "edit_total":
		return BudgetManageOpEditTotal, nil
	case "edit_distribution":
		return BudgetManageOpEditDistribution, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidBudgetManageOperationKind, s)
	}
}

type BudgetManageAwaitingSlot int

const (
	BudgetManageAwaitingTotal BudgetManageAwaitingSlot = iota + 1
	BudgetManageAwaitingDistribution
	BudgetManageAwaitingConfirm
)

func (a BudgetManageAwaitingSlot) String() string {
	switch a {
	case BudgetManageAwaitingTotal:
		return "total"
	case BudgetManageAwaitingDistribution:
		return "distribution"
	case BudgetManageAwaitingConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a BudgetManageAwaitingSlot) IsValid() bool {
	return a >= BudgetManageAwaitingTotal && a <= BudgetManageAwaitingConfirm
}

var errInvalidBudgetManageAwaitingSlot = errors.New("workflows: budget manage awaiting slot inválido")

func ParseBudgetManageAwaitingSlot(s string) (BudgetManageAwaitingSlot, error) {
	switch s {
	case "total":
		return BudgetManageAwaitingTotal, nil
	case "distribution":
		return BudgetManageAwaitingDistribution, nil
	case "confirm":
		return BudgetManageAwaitingConfirm, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidBudgetManageAwaitingSlot, s)
	}
}

type BudgetManageStatus int

const (
	BudgetManageActive BudgetManageStatus = iota + 1
	BudgetManageCompleted
	BudgetManageCancelled
	BudgetManageExpired
)

func (b BudgetManageStatus) String() string {
	switch b {
	case BudgetManageActive:
		return "active"
	case BudgetManageCompleted:
		return "completed"
	case BudgetManageCancelled:
		return "cancelled"
	case BudgetManageExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (b BudgetManageStatus) IsValid() bool {
	return b >= BudgetManageActive && b <= BudgetManageExpired
}

var errInvalidBudgetManageStatus = errors.New("workflows: budget manage status inválido")

func ParseBudgetManageStatus(s string) (BudgetManageStatus, error) {
	switch s {
	case "active":
		return BudgetManageActive, nil
	case "completed":
		return BudgetManageCompleted, nil
	case "cancelled":
		return BudgetManageCancelled, nil
	case "expired":
		return BudgetManageExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidBudgetManageStatus, s)
	}
}

type BudgetManageState struct {
	Status              BudgetManageStatus        `json:"status"`
	Operation           BudgetManageOperationKind `json:"operation"`
	Awaiting            BudgetManageAwaitingSlot  `json:"awaiting"`
	UserID              uuid.UUID                 `json:"userId"`
	Competence          string                    `json:"competence"`
	TotalCents          int64                     `json:"totalCents"`
	PreviousTotalCents  int64                     `json:"previousTotalCents"`
	Allocations         map[string]int            `json:"allocations"`
	PreviousAllocations map[string]int            `json:"previousAllocations"`
	ResumeText          string                    `json:"resumeText"`
	ResponseText        string                    `json:"responseText"`
	RepromptCount       int                       `json:"repromptCount"`
	MessageID           string                    `json:"messageId"`
	IncomingMessageID   string                    `json:"incomingMessageId"`
	SuspendedAt         time.Time                 `json:"suspendedAt"`
	Expired             bool                      `json:"expired"`
}
