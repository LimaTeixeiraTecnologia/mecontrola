package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

var (
	errInvalidPendingStatus        = errors.New("workflows: pending status inválido")
	errInvalidAwaitingSlot         = errors.New("workflows: awaiting slot inválido")
	errInvalidPendingOperationKind = errors.New("workflows: pending operation kind inválido")
)

type PendingStatus int

const (
	PendingStatusActive PendingStatus = iota + 1
	PendingStatusCompleted
	PendingStatusCancelled
	PendingStatusExpired
	PendingStatusReplaced
)

func (p PendingStatus) String() string {
	switch p {
	case PendingStatusActive:
		return "active"
	case PendingStatusCompleted:
		return "completed"
	case PendingStatusCancelled:
		return "cancelled"
	case PendingStatusExpired:
		return "expired"
	case PendingStatusReplaced:
		return "replaced"
	default:
		return "unknown"
	}
}

func (p PendingStatus) IsValid() bool {
	return p >= PendingStatusActive && p <= PendingStatusReplaced
}

func ParsePendingStatus(s string) (PendingStatus, error) {
	switch s {
	case "active":
		return PendingStatusActive, nil
	case "completed":
		return PendingStatusCompleted, nil
	case "cancelled":
		return PendingStatusCancelled, nil
	case "expired":
		return PendingStatusExpired, nil
	case "replaced":
		return PendingStatusReplaced, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidPendingStatus, s)
	}
}

type AwaitingSlot int

const (
	AwaitingSlotCategory AwaitingSlot = iota + 1
	AwaitingSlotPaymentMethod
	AwaitingSlotCard
	AwaitingSlotDate
	AwaitingSlotConfirmation
	AwaitingSlotCorrection
)

func (a AwaitingSlot) String() string {
	switch a {
	case AwaitingSlotCategory:
		return "category"
	case AwaitingSlotPaymentMethod:
		return "payment_method"
	case AwaitingSlotCard:
		return "card"
	case AwaitingSlotDate:
		return "date"
	case AwaitingSlotConfirmation:
		return "confirmation"
	case AwaitingSlotCorrection:
		return "correction"
	default:
		return "unknown"
	}
}

func (a AwaitingSlot) IsValid() bool {
	return a >= AwaitingSlotCategory && a <= AwaitingSlotCorrection
}

func ParseAwaitingSlot(s string) (AwaitingSlot, error) {
	switch s {
	case "category":
		return AwaitingSlotCategory, nil
	case "payment_method":
		return AwaitingSlotPaymentMethod, nil
	case "card":
		return AwaitingSlotCard, nil
	case "date":
		return AwaitingSlotDate, nil
	case "confirmation":
		return AwaitingSlotConfirmation, nil
	case "correction":
		return AwaitingSlotCorrection, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidAwaitingSlot, s)
	}
}

type PendingOperationKind int

const (
	PendingOpRegisterExpense PendingOperationKind = iota + 1
	PendingOpRegisterIncome
	PendingOpEditEntry
	PendingOpCreateRecurrence
)

func (p PendingOperationKind) String() string {
	switch p {
	case PendingOpRegisterExpense:
		return "register_expense"
	case PendingOpRegisterIncome:
		return "register_income"
	case PendingOpEditEntry:
		return "edit_entry"
	case PendingOpCreateRecurrence:
		return "create_recurrence"
	default:
		return "unknown"
	}
}

func (p PendingOperationKind) IsValid() bool {
	return p >= PendingOpRegisterExpense && p <= PendingOpCreateRecurrence
}

func ParsePendingOperationKind(s string) (PendingOperationKind, error) {
	switch s {
	case "register_expense":
		return PendingOpRegisterExpense, nil
	case "register_income":
		return PendingOpRegisterIncome, nil
	case "edit_entry":
		return PendingOpEditEntry, nil
	case "create_recurrence":
		return PendingOpCreateRecurrence, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidPendingOperationKind, s)
	}
}

type PendingOperation struct {
	Kind PendingOperationKind `json:"kind"`
}

type PendingEntryState struct {
	Status               PendingStatus              `json:"status"`
	Awaiting             AwaitingSlot               `json:"awaiting"`
	Operation            PendingOperation           `json:"operation"`
	UserID               uuid.UUID                  `json:"userId"`
	ThreadID             string                     `json:"threadId"`
	MessageID            string                     `json:"messageId"`
	OriginalText         string                     `json:"originalText"`
	ResumeText           string                     `json:"resumeText"`
	AmountCents          int64                      `json:"amountCents"`
	Description          string                     `json:"description"`
	PaymentMethod        string                     `json:"paymentMethod"`
	CardID               *uuid.UUID                 `json:"cardId"`
	Installments         int                        `json:"installments"`
	OccurredAt           string                     `json:"occurredAt"`
	Kind                 interfaces.CategoryKind    `json:"kind"`
	Candidates           []PendingCategoryCandidate `json:"candidates"`
	CategoryVersion      int64                      `json:"categoryVersion"`
	RepromptCount        int                        `json:"repromptCount"`
	SuspendedAt          time.Time                  `json:"suspendedAt"`
	ResponseText         string                     `json:"responseText"`
	ResourceID           uuid.UUID                  `json:"resourceId"`
	ErrorCode            string                     `json:"errorCode"`
	OperationKind        PendingOperationKind       `json:"operationKind"`
	TargetTransactionID  *uuid.UUID                 `json:"targetTransactionId"`
	TargetVersion        int64                      `json:"targetVersion"`
	Frequency            string                     `json:"frequency"`
	RecurrenceDayOfMonth int                        `json:"recurrenceDayOfMonth"`
	ConfirmRepromptCount int                        `json:"confirmRepromptCount"`
	IncomingMessageID    string                     `json:"incomingMessageId"`
	ProcessedMessageID   string                     `json:"processedMessageId"`
	ItemSeq              int                        `json:"itemSeq"`
}
