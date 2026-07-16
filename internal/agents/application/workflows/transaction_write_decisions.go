package workflows

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

var ErrTransactionWriteAcceptedWithoutResource = ErrWriteAcceptedWithoutResource

const (
	transactionWriteTTL       = 30 * time.Minute
	transactionMaxReprompts   = 1
	transactionCreditCardCode = PaymentMethodCreditCard
)

func DecideTransactionPostWrite(outcome agent.ToolOutcome, resourceID uuid.UUID) (TransactionWriteStatus, workflow.StepStatus, error) {
	if outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil {
		return TransactionWriteStatusActive, workflow.StepStatusFailed, ErrTransactionWriteAcceptedWithoutResource
	}
	return TransactionWriteStatusCompleted, workflow.StepStatusCompleted, nil
}

func isTransactionExpired(state TransactionWriteState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > transactionWriteTTL
}

type initialAwaitingArgs struct {
	CategoryAwaiting TransactionAwaitingSlot
	PaymentMethod    string
	HasCard          bool
	HasEditCandidate bool
	EditCandidates   int
}

type decideInitialAwaitingFn func(initialAwaitingArgs) TransactionAwaitingSlot

var initialAwaitingByOperation = map[TransactionOperationKind]decideInitialAwaitingFn{
	TransactionOpRegisterExpense:  decideInitialAwaitingRegisterExpense,
	TransactionOpRegisterIncome:   decideInitialAwaitingRegisterIncome,
	TransactionOpCreateRecurrence: decideInitialAwaitingRegisterExpense,
	TransactionOpEditEntry:        decideInitialAwaitingEditEntry,
}

func decideInitialAwaitingRegisterExpense(args initialAwaitingArgs) TransactionAwaitingSlot {
	if args.CategoryAwaiting == TransactionAwaitingCategory {
		return TransactionAwaitingCategory
	}
	if args.PaymentMethod == "" {
		return TransactionAwaitingPaymentMethod
	}
	if args.PaymentMethod == transactionCreditCardCode && !args.HasCard {
		return TransactionAwaitingCard
	}
	return TransactionAwaitingConfirmation
}

func decideInitialAwaitingRegisterIncome(args initialAwaitingArgs) TransactionAwaitingSlot {
	if args.CategoryAwaiting == TransactionAwaitingCategory {
		return TransactionAwaitingCategory
	}
	return TransactionAwaitingConfirmation
}

func decideInitialAwaitingEditEntry(args initialAwaitingArgs) TransactionAwaitingSlot {
	if args.EditCandidates > 1 {
		return TransactionAwaitingEditCandidate
	}
	return TransactionAwaitingConfirmation
}

func DecideTransactionInitialAwaiting(kind TransactionOperationKind, args initialAwaitingArgs) TransactionAwaitingSlot {
	fn, ok := initialAwaitingByOperation[kind]
	if !ok {
		return TransactionAwaitingConfirmation
	}
	return fn(args)
}

type TransactionSlotAction int

const (
	TransactionSlotActionNone TransactionSlotAction = iota + 1
	TransactionSlotActionExpire
	TransactionSlotActionCancel
	TransactionSlotActionReplace
	TransactionSlotActionFill
	TransactionSlotActionReprompt
)

type TransactionSlotDecision struct {
	Action      TransactionSlotAction
	Slot        TransactionAwaitingSlot
	FilledValue string
}

func DecideTransactionSlotResume(state TransactionWriteState, text string, now time.Time) TransactionSlotDecision {
	if isTransactionExpired(state, now) {
		return TransactionSlotDecision{Action: TransactionSlotActionExpire}
	}

	trimmed := strings.TrimSpace(text)

	if isCancelMessage(trimmed) {
		return TransactionSlotDecision{Action: TransactionSlotActionCancel}
	}

	if isNewCompleteOperation(trimmed) {
		return TransactionSlotDecision{Action: TransactionSlotActionReplace}
	}

	switch state.Awaiting {
	case TransactionAwaitingPaymentMethod:
		if pm := recognizePaymentMethod(trimmed); pm != "" {
			return TransactionSlotDecision{Action: TransactionSlotActionFill, Slot: TransactionAwaitingPaymentMethod, FilledValue: pm}
		}
	case TransactionAwaitingDate:
		if d := parseInputDate(trimmed, now); d != "" {
			return TransactionSlotDecision{Action: TransactionSlotActionFill, Slot: TransactionAwaitingDate, FilledValue: d}
		}
	case TransactionAwaitingCard:
		return TransactionSlotDecision{Action: TransactionSlotActionFill, Slot: TransactionAwaitingCard, FilledValue: trimmed}
	}

	if state.RepromptCount >= transactionMaxReprompts {
		return TransactionSlotDecision{Action: TransactionSlotActionCancel}
	}

	return TransactionSlotDecision{Action: TransactionSlotActionReprompt}
}

type TransactionConfirmAction int

const (
	TransactionConfirmActionAccept TransactionConfirmAction = iota + 1
	TransactionConfirmActionCancel
	TransactionConfirmActionReprompt
	TransactionConfirmActionExpire
	TransactionConfirmActionReplay
)

func DecideTransactionConfirmation(state TransactionWriteState, msg PendingMessage, now time.Time) TransactionConfirmAction {
	if isTransactionExpired(state, now) {
		return TransactionConfirmActionExpire
	}

	if msg.MessageID != "" && msg.MessageID == state.ProcessedMessageID {
		return TransactionConfirmActionReplay
	}

	text := strings.TrimSpace(msg.Text)

	if reConfirmYes.MatchString(text) {
		return TransactionConfirmActionAccept
	}

	if reConfirmNo.MatchString(text) || isCancelMessage(text) {
		return TransactionConfirmActionCancel
	}

	if state.ConfirmRepromptCount >= transactionMaxReprompts {
		return TransactionConfirmActionCancel
	}

	return TransactionConfirmActionReprompt
}

func DecideEditCandidateChoice(candidates []TransactionEditCandidate, text string) (int, bool) {
	trimmed := strings.TrimSpace(text)
	if idx, err := strconv.Atoi(trimmed); err == nil {
		if idx >= 1 && idx <= len(candidates) {
			return idx - 1, true
		}
		return 0, false
	}
	return 0, false
}

func DecideNewTransactionOperationReplacement(text string) bool {
	return isNewCompleteOperation(strings.TrimSpace(text))
}

func DecideTransactionCategoryChoice(candidates []PendingCategoryCandidate, text string) (CategoryChoiceDecision, error) {
	trimmed := strings.TrimSpace(text)

	if idx, err := strconv.Atoi(trimmed); err == nil {
		if idx >= 1 && idx <= len(candidates) {
			c := candidates[idx-1]
			if c.SubcategoryID == (uuid.UUID{}) || c.SubcategoryID == c.RootCategoryID {
				return CategoryChoiceDecision{Action: CategoryChoiceActionRootOnly, Candidate: c}, nil
			}
			return CategoryChoiceDecision{Action: CategoryChoiceActionSelected, Candidate: c}, nil
		}
		return CategoryChoiceDecision{Action: CategoryChoiceActionReprompt}, nil
	}

	normalized := normalizeText(trimmed)
	var matches []PendingCategoryCandidate
	for _, c := range candidates {
		if normalizeText(c.SubcategorySlug) == normalized || normalizeText(c.Path) == normalized {
			matches = append(matches, c)
		}
	}

	if len(matches) == 1 {
		c := matches[0]
		if c.SubcategoryID == (uuid.UUID{}) || c.SubcategoryID == c.RootCategoryID || c.SubcategorySlug == "" {
			return CategoryChoiceDecision{Action: CategoryChoiceActionRootOnly, Candidate: c}, nil
		}
		return CategoryChoiceDecision{Action: CategoryChoiceActionSelected, Candidate: c}, nil
	}

	if len(matches) > 1 {
		return CategoryChoiceDecision{Action: CategoryChoiceActionAmbiguous}, nil
	}

	return CategoryChoiceDecision{Action: CategoryChoiceActionReprompt}, nil
}
