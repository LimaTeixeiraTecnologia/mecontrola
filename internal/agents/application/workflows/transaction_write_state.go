package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

var (
	errInvalidTransactionWriteStatus  = errors.New("workflows: transaction write status inválido")
	errInvalidTransactionAwaitingSlot = errors.New("workflows: transaction awaiting slot inválido")
	errInvalidTransactionOperation    = errors.New("workflows: transaction operation kind inválido")
)

type TransactionWriteStatus int

const (
	TransactionWriteStatusActive TransactionWriteStatus = iota + 1
	TransactionWriteStatusCompleted
	TransactionWriteStatusCancelled
	TransactionWriteStatusExpired
	TransactionWriteStatusReplaced
)

func (s TransactionWriteStatus) String() string {
	switch s {
	case TransactionWriteStatusActive:
		return "active"
	case TransactionWriteStatusCompleted:
		return "completed"
	case TransactionWriteStatusCancelled:
		return "cancelled"
	case TransactionWriteStatusExpired:
		return "expired"
	case TransactionWriteStatusReplaced:
		return "replaced"
	default:
		return "unknown"
	}
}

func (s TransactionWriteStatus) IsValid() bool {
	return s >= TransactionWriteStatusActive && s <= TransactionWriteStatusReplaced
}

func ParseTransactionWriteStatus(s string) (TransactionWriteStatus, error) {
	switch s {
	case "active":
		return TransactionWriteStatusActive, nil
	case "completed":
		return TransactionWriteStatusCompleted, nil
	case "cancelled":
		return TransactionWriteStatusCancelled, nil
	case "expired":
		return TransactionWriteStatusExpired, nil
	case "replaced":
		return TransactionWriteStatusReplaced, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidTransactionWriteStatus, s)
	}
}

type TransactionAwaitingSlot int

const (
	TransactionAwaitingCategory TransactionAwaitingSlot = iota + 1
	TransactionAwaitingPaymentMethod
	TransactionAwaitingCard
	TransactionAwaitingDate
	TransactionAwaitingEditCandidate
	TransactionAwaitingConfirmation
)

func (a TransactionAwaitingSlot) String() string {
	switch a {
	case TransactionAwaitingCategory:
		return "category"
	case TransactionAwaitingPaymentMethod:
		return "payment_method"
	case TransactionAwaitingCard:
		return "card"
	case TransactionAwaitingDate:
		return "date"
	case TransactionAwaitingEditCandidate:
		return "edit_candidate"
	case TransactionAwaitingConfirmation:
		return "confirmation"
	default:
		return "unknown"
	}
}

func (a TransactionAwaitingSlot) IsValid() bool {
	return a >= TransactionAwaitingCategory && a <= TransactionAwaitingConfirmation
}

func ParseTransactionAwaitingSlot(s string) (TransactionAwaitingSlot, error) {
	switch s {
	case "category":
		return TransactionAwaitingCategory, nil
	case "payment_method":
		return TransactionAwaitingPaymentMethod, nil
	case "card":
		return TransactionAwaitingCard, nil
	case "date":
		return TransactionAwaitingDate, nil
	case "edit_candidate":
		return TransactionAwaitingEditCandidate, nil
	case "confirmation":
		return TransactionAwaitingConfirmation, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidTransactionAwaitingSlot, s)
	}
}

type TransactionOperationKind int

const (
	TransactionOpRegisterExpense TransactionOperationKind = iota + 1
	TransactionOpRegisterIncome
	TransactionOpEditEntry
	TransactionOpCreateRecurrence
)

func (o TransactionOperationKind) String() string {
	switch o {
	case TransactionOpRegisterExpense:
		return "register_expense"
	case TransactionOpRegisterIncome:
		return "register_income"
	case TransactionOpEditEntry:
		return "edit_entry"
	case TransactionOpCreateRecurrence:
		return "create_recurrence"
	default:
		return "unknown"
	}
}

func (o TransactionOperationKind) IsValid() bool {
	return o >= TransactionOpRegisterExpense && o <= TransactionOpCreateRecurrence
}

func ParseTransactionOperationKind(s string) (TransactionOperationKind, error) {
	switch s {
	case "register_expense":
		return TransactionOpRegisterExpense, nil
	case "register_income":
		return TransactionOpRegisterIncome, nil
	case "edit_entry":
		return TransactionOpEditEntry, nil
	case "create_recurrence":
		return TransactionOpCreateRecurrence, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidTransactionOperation, s)
	}
}

type TransactionEditCandidate struct {
	TransactionID           uuid.UUID  `json:"transactionId"`
	AmountCents             int64      `json:"amountCents"`
	Description             string     `json:"description"`
	CategoryID              uuid.UUID  `json:"categoryId"`
	SubcategoryID           *uuid.UUID `json:"subcategoryId"`
	CategoryNameSnapshot    string     `json:"categoryNameSnapshot"`
	SubcategoryNameSnapshot string     `json:"subcategoryNameSnapshot"`
	PaymentMethod           string     `json:"paymentMethod"`
	OccurredAt              string     `json:"occurredAt"`
	Version                 int64      `json:"version"`
}

type TransactionWriteState struct {
	Status        TransactionWriteStatus   `json:"status"`
	Awaiting      TransactionAwaitingSlot  `json:"awaiting"`
	OperationKind TransactionOperationKind `json:"operationKind"`

	UserID       uuid.UUID `json:"userId"`
	ResourceID   uuid.UUID `json:"resourceId"`
	ThreadID     string    `json:"threadId"`
	MessageID    string    `json:"messageId"`
	OriginalText string    `json:"originalText"`
	ResumeText   string    `json:"resumeText"`

	AmountCents   int64                   `json:"amountCents"`
	Description   string                  `json:"description"`
	PaymentMethod string                  `json:"paymentMethod"`
	CardID        *uuid.UUID              `json:"cardId"`
	Installments  int                     `json:"installments"`
	OccurredAt    string                  `json:"occurredAt"`
	Kind          interfaces.CategoryKind `json:"kind"`

	Frequency            string `json:"frequency"`
	RecurrenceDayOfMonth int    `json:"recurrenceDayOfMonth"`

	Candidates      []PendingCategoryCandidate `json:"candidates"`
	CategoryVersion int64                      `json:"categoryVersion"`

	TargetTransactionID   *uuid.UUID                 `json:"targetTransactionId"`
	TargetVersion         int64                      `json:"targetVersion"`
	TargetCategoryID      uuid.UUID                  `json:"targetCategoryId"`
	TargetSubcategoryID   *uuid.UUID                 `json:"targetSubcategoryId"`
	TargetPaymentMethod   string                     `json:"targetPaymentMethod"`
	TargetDescription     string                     `json:"targetDescription"`
	TargetOccurredAt      string                     `json:"targetOccurredAt"`
	EditSearchAmountCents int64                      `json:"editSearchAmountCents"`
	EditSearchTerm        string                     `json:"editSearchTerm"`
	EditCandidates        []TransactionEditCandidate `json:"editCandidates"`

	EditPreviousAmountCents  int64  `json:"editPreviousAmountCents"`
	EditPreviousCategory     string `json:"editPreviousCategory"`
	EditPreviousPayment      string `json:"editPreviousPayment"`
	EditPaymentMethodChanged bool   `json:"editPaymentMethodChanged"`

	RepromptCount        int `json:"repromptCount"`
	ConfirmRepromptCount int `json:"confirmRepromptCount"`

	SuspendedAt  time.Time `json:"suspendedAt"`
	ResponseText string    `json:"responseText"`
	ErrorCode    string    `json:"errorCode"`

	IncomingMessageID      string `json:"incomingMessageId"`
	ProcessedMessageID     string `json:"processedMessageId"`
	ItemSeq                int    `json:"itemSeq"`
	FailedWriteResumeCount int    `json:"failedWriteResumeCount"`
}
