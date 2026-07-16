package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

const (
	TransactionWriteWorkflowID  = "transaction-write"
	stepTransactionWriteID      = "transaction-write"
	TransactionWriteStaleAfter  = 35 * time.Minute
	TransactionWriteReaperBatch = 100

	originOperationTransactionWrite = "transaction_write_register"
	transactionCategorySrcUser      = "user_selected_candidate"
	defaultEditCandidateLimit       = 5
)

func TransactionWriteKey(resourceID, threadID string) string {
	return resourceID + ":" + threadID + ":" + TransactionWriteWorkflowID
}

const transactionWriteFalseSuccessMetric = "agents_transaction_write_false_success_total"

type transactionWriteMetrics struct {
	falseSuccess observability.Counter
}

func BuildTransactionWriteWorkflowWithObservability(ledger interfaces.TransactionsLedger, cards cardNicknameSolver, cats categoryValidator, idem IdempotentWriter, o11y observability.Observability) workflow.Definition[TransactionWriteState] {
	var metrics transactionWriteMetrics
	if o11y != nil {
		metrics.falseSuccess = o11y.Metrics().Counter(
			transactionWriteFalseSuccessMetric,
			"Confirmacao positiva sem transacao ativa no workflow transaction-write",
			"1",
		)
	}
	step := workflow.NewStepFunc(stepTransactionWriteID, makeTransactionWriteStep(ledger, cards, cats, idem, &metrics))
	return workflow.Definition[TransactionWriteState]{
		ID:          TransactionWriteWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildTransactionWriteReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, TransactionWriteWorkflowID, TransactionWriteStaleAfter, TransactionWriteReaperBatch, o11y)
}

func makeTransactionWriteStep(ledger interfaces.TransactionsLedger, cards cardNicknameSolver, cats categoryValidator, idem IdempotentWriter, metrics *transactionWriteMetrics) func(context.Context, TransactionWriteState) (workflow.StepOutput[TransactionWriteState], error) {
	return func(ctx context.Context, state TransactionWriteState) (workflow.StepOutput[TransactionWriteState], error) {
		if state.ResumeText == "" {
			return handleTransactionInitial(ctx, state, ledger)
		}

		now := time.Now().UTC()

		switch state.Awaiting {
		case TransactionAwaitingCategory:
			return handleTransactionCategoryResume(ctx, state, now, cats)
		case TransactionAwaitingCard:
			return handleTransactionCardResume(ctx, state, now, cards)
		case TransactionAwaitingEditCandidate:
			return handleTransactionEditCandidateResume(state, now)
		case TransactionAwaitingConfirmation:
			return handleTransactionConfirmationResume(ctx, state, now, ledger, cats, idem, metrics)
		default:
			return handleTransactionSlotResume(state, now)
		}
	}
}

func suspendTransaction(state TransactionWriteState, prompt string) (workflow.StepOutput[TransactionWriteState], error) {
	state.SuspendedAt = time.Now().UTC()
	state.ResumeText = ""
	state.ResponseText = prompt
	return workflow.StepOutput[TransactionWriteState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}, nil
}

func completeTransaction(state TransactionWriteState, status TransactionWriteStatus, response string) (workflow.StepOutput[TransactionWriteState], error) {
	state.Status = status
	state.ResponseText = response
	state.ResumeText = ""
	return workflow.StepOutput[TransactionWriteState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func handleTransactionInitial(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger) (workflow.StepOutput[TransactionWriteState], error) {
	if state.OperationKind == TransactionOpEditEntry && state.TargetTransactionID == nil {
		return handleTransactionEditSearch(ctx, state, ledger)
	}

	if state.Awaiting == 0 {
		state.Awaiting = DecideTransactionInitialAwaiting(state.OperationKind, initialAwaitingArgs{
			CategoryAwaiting: categoryAwaitingFromCandidates(state.Candidates),
			PaymentMethod:    state.PaymentMethod,
			HasCard:          state.CardID != nil,
		})
	}

	if state.Awaiting == TransactionAwaitingConfirmation {
		return suspendTransaction(state, buildTransactionConfirmSummary(state))
	}

	return suspendTransaction(state, buildTransactionSlotPrompt(state))
}

func categoryAwaitingFromCandidates(candidates []PendingCategoryCandidate) TransactionAwaitingSlot {
	if len(candidates) == 1 {
		return TransactionAwaitingConfirmation
	}
	return TransactionAwaitingCategory
}

func handleTransactionEditSearch(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger) (workflow.StepOutput[TransactionWriteState], error) {
	entries, err := ledger.SearchEditCandidates(ctx, state.UserID, interfaces.EditCandidateQuery{
		AmountCents: state.EditSearchAmountCents,
		Term:        state.EditSearchTerm,
		RefMonth:    "",
		Limit:       defaultEditCandidateLimit,
	})
	if err != nil {
		return workflow.StepOutput[TransactionWriteState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("workflows.transaction_write: search_edit_candidates: %w", err)
	}

	candidates := make([]TransactionEditCandidate, 0, len(entries))
	for _, e := range entries {
		candidates = append(candidates, transactionEditCandidateFromEntry(e))
	}

	if len(candidates) == 0 {
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.NoEditCandidateFound())
	}

	if len(candidates) == 1 {
		return promoteEditCandidateToConfirmation(state, candidates[0])
	}

	state.EditCandidates = candidates
	state.Awaiting = TransactionAwaitingEditCandidate
	return suspendTransaction(state, buildEditCandidatesPrompt(candidates))
}

func transactionEditCandidateFromEntry(e interfaces.Entry) TransactionEditCandidate {
	txID, _ := uuid.Parse(e.ID)
	catID, _ := uuid.Parse(e.CategoryID)
	var sub *uuid.UUID
	if e.SubcategoryID != nil {
		if parsed, parseErr := uuid.Parse(*e.SubcategoryID); parseErr == nil {
			sub = &parsed
		}
	}
	return TransactionEditCandidate{
		TransactionID:           txID,
		AmountCents:             e.AmountCents,
		Description:             e.Description,
		CategoryID:              catID,
		SubcategoryID:           sub,
		CategoryNameSnapshot:    e.CategoryNameSnapshot,
		SubcategoryNameSnapshot: e.SubcategoryNameSnapshot,
		PaymentMethod:           e.PaymentMethod,
		OccurredAt:              e.OccurredAt.Format("2006-01-02"),
		Version:                 e.Version,
	}
}

func buildEditCandidatesPrompt(candidates []TransactionEditCandidate) string {
	paths := make([]string, 0, len(candidates))
	for _, c := range candidates {
		label := c.SubcategoryNameSnapshot
		if label == "" {
			label = c.CategoryNameSnapshot
		}
		paths = append(paths, fmt.Sprintf("%s - %s", money.FromCents(c.AmountCents).BRL(), label))
	}
	return messages.EditCandidatesPrompt(paths)
}

func handleTransactionEditCandidateResume(state TransactionWriteState, now time.Time) (workflow.StepOutput[TransactionWriteState], error) {
	if isTransactionExpired(state, now) {
		return completeTransaction(state, TransactionWriteStatusExpired, messages.WriteExpired())
	}

	text := strings.TrimSpace(state.ResumeText)

	if isCancelMessage(text) {
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())
	}

	idx, ok := DecideEditCandidateChoice(state.EditCandidates, text)
	if !ok {
		if state.RepromptCount >= transactionMaxReprompts {
			return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())
		}
		state.RepromptCount++
		return suspendTransaction(state, buildEditCandidatesPrompt(state.EditCandidates))
	}

	return promoteEditCandidateToConfirmation(state, state.EditCandidates[idx])
}

func promoteEditCandidateToConfirmation(state TransactionWriteState, candidate TransactionEditCandidate) (workflow.StepOutput[TransactionWriteState], error) {
	txID := candidate.TransactionID
	state.TargetTransactionID = &txID
	state.TargetVersion = candidate.Version
	state.TargetCategoryID = candidate.CategoryID
	state.TargetSubcategoryID = candidate.SubcategoryID
	state.TargetPaymentMethod = candidate.PaymentMethod
	state.TargetDescription = candidate.Description
	state.TargetOccurredAt = candidate.OccurredAt

	state.EditPreviousAmountCents = candidate.AmountCents
	label := candidate.SubcategoryNameSnapshot
	if label == "" {
		label = candidate.CategoryNameSnapshot
	}
	state.EditPreviousCategory = label
	state.EditPreviousPayment = formatPaymentLabel(candidate.PaymentMethod)

	if state.AmountCents == 0 {
		state.AmountCents = candidate.AmountCents
	}
	if state.Description == "" {
		state.Description = candidate.Description
	}
	if state.OccurredAt == "" {
		state.OccurredAt = candidate.OccurredAt
	}
	if state.PaymentMethod != "" && state.PaymentMethod != candidate.PaymentMethod {
		state.EditPaymentMethodChanged = true
	}

	state.RepromptCount = 0
	state.EditCandidates = nil
	state.Awaiting = TransactionAwaitingConfirmation
	return suspendTransaction(state, buildTransactionConfirmSummary(state))
}

func handleTransactionCategoryResume(ctx context.Context, state TransactionWriteState, now time.Time, cats categoryValidator) (workflow.StepOutput[TransactionWriteState], error) {
	if isTransactionExpired(state, now) {
		return completeTransaction(state, TransactionWriteStatusExpired, messages.WriteExpired())
	}

	text := strings.TrimSpace(state.ResumeText)

	if isCancelMessage(text) {
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())
	}

	if isNewCompleteOperation(text) {
		return completeTransaction(state, TransactionWriteStatusReplaced, "")
	}

	if len(state.Candidates) > 0 {
		decision, decErr := DecideTransactionCategoryChoice(state.Candidates, text)
		if decErr == nil && decision.Action == CategoryChoiceActionSelected {
			return promoteTransactionCategoryToConfirmation(state, decision.Candidate)
		}
		if decErr == nil && decision.Action == CategoryChoiceActionRootOnly {
			return transactionCategoryReprompt(state, "Essa categoria precisa de uma subcategoria específica. Qual você quer usar?")
		}
	}

	if cats != nil {
		candidates, searchErr := SearchAndEnrichCandidates(ctx, cats, text, state.Kind, state.CategoryVersion)
		if searchErr == nil && len(candidates) == 1 {
			return promoteTransactionCategoryToConfirmation(state, candidates[0])
		}
		if searchErr == nil && len(candidates) > 1 {
			state.Candidates = candidates
			state.RepromptCount = 0
			return suspendTransaction(state, buildCandidatesPrompt(candidates))
		}
	}

	return transactionCategoryReprompt(state, buildTransactionSlotReprompt(state))
}

func promoteTransactionCategoryToConfirmation(state TransactionWriteState, candidate PendingCategoryCandidate) (workflow.StepOutput[TransactionWriteState], error) {
	state.Candidates = []PendingCategoryCandidate{candidate}
	state.RepromptCount = 0

	next := DecideTransactionInitialAwaiting(state.OperationKind, initialAwaitingArgs{
		CategoryAwaiting: TransactionAwaitingConfirmation,
		PaymentMethod:    state.PaymentMethod,
		HasCard:          state.CardID != nil,
	})
	state.Awaiting = next

	if next == TransactionAwaitingConfirmation {
		return suspendTransaction(state, buildTransactionConfirmSummary(state))
	}
	return suspendTransaction(state, buildTransactionSlotPrompt(state))
}

func transactionCategoryReprompt(state TransactionWriteState, prompt string) (workflow.StepOutput[TransactionWriteState], error) {
	if state.RepromptCount >= transactionMaxReprompts {
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())
	}
	state.RepromptCount++
	return suspendTransaction(state, prompt)
}

func handleTransactionCardResume(ctx context.Context, state TransactionWriteState, now time.Time, cards cardNicknameSolver) (workflow.StepOutput[TransactionWriteState], error) {
	if isTransactionExpired(state, now) {
		return completeTransaction(state, TransactionWriteStatusExpired, messages.WriteExpired())
	}

	text := strings.TrimSpace(state.ResumeText)

	if isCancelMessage(text) {
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())
	}

	if isNewCompleteOperation(text) {
		return completeTransaction(state, TransactionWriteStatusReplaced, "")
	}

	if cards != nil {
		card, err := cards.ResolveCardByNickname(ctx, state.UserID, text)
		if err == nil {
			cardUUID, parseErr := uuid.Parse(card.ID)
			if parseErr == nil {
				state.CardID = &cardUUID
				state.RepromptCount = 0
				state.Awaiting = TransactionAwaitingConfirmation
				return suspendTransaction(state, buildTransactionConfirmSummary(state))
			}
		}
	}

	if state.RepromptCount >= transactionMaxReprompts {
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())
	}

	state.RepromptCount++
	return suspendTransaction(state, messages.CardPrompt())
}

func handleTransactionSlotResume(state TransactionWriteState, now time.Time) (workflow.StepOutput[TransactionWriteState], error) {
	decision := DecideTransactionSlotResume(state, state.ResumeText, now)

	switch decision.Action {
	case TransactionSlotActionExpire:
		return completeTransaction(state, TransactionWriteStatusExpired, messages.WriteExpired())

	case TransactionSlotActionCancel:
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())

	case TransactionSlotActionReplace:
		return completeTransaction(state, TransactionWriteStatusReplaced, "")

	case TransactionSlotActionReprompt:
		state.RepromptCount++
		return suspendTransaction(state, buildTransactionSlotReprompt(state))

	case TransactionSlotActionFill:
		if decision.Slot == TransactionAwaitingPaymentMethod && decision.FilledValue != "" {
			state.PaymentMethod = decision.FilledValue
		}
		if decision.Slot == TransactionAwaitingDate && decision.FilledValue != "" {
			state.OccurredAt = decision.FilledValue
		}
		state.RepromptCount = 0

		next := DecideTransactionInitialAwaiting(state.OperationKind, initialAwaitingArgs{
			CategoryAwaiting: TransactionAwaitingConfirmation,
			PaymentMethod:    state.PaymentMethod,
			HasCard:          state.CardID != nil,
		})
		state.Awaiting = next
		if next == TransactionAwaitingConfirmation {
			return suspendTransaction(state, buildTransactionConfirmSummary(state))
		}
		return suspendTransaction(state, buildTransactionSlotPrompt(state))

	default:
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteFailure())
	}
}

func handleTransactionConfirmationResume(ctx context.Context, state TransactionWriteState, now time.Time, ledger interfaces.TransactionsLedger, cats categoryValidator, idem IdempotentWriter, metrics *transactionWriteMetrics) (workflow.StepOutput[TransactionWriteState], error) {
	msg := PendingMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
	action := DecideTransactionConfirmation(state, msg, now)

	switch action {
	case TransactionConfirmActionAccept:
		state.ProcessedMessageID = state.IncomingMessageID
		return executeTransactionWrite(ctx, state, ledger, cats, idem, metrics)

	case TransactionConfirmActionCancel:
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteCancelled())

	case TransactionConfirmActionExpire:
		return completeTransaction(state, TransactionWriteStatusExpired, messages.WriteExpired())

	case TransactionConfirmActionReplay:
		state.ResumeText = ""
		return workflow.StepOutput[TransactionWriteState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case TransactionConfirmActionReprompt:
		state.ConfirmRepromptCount++
		state.ProcessedMessageID = msg.MessageID
		return suspendTransaction(state, messages.ConfirmationReprompt())

	default:
		return completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteFailure())
	}
}

func executeTransactionWrite(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger, cats categoryValidator, idem IdempotentWriter, metrics *transactionWriteMetrics) (workflow.StepOutput[TransactionWriteState], error) {
	if state.OperationKind != TransactionOpEditEntry || len(state.Candidates) > 0 {
		newState, ok, override, err := validateTransactionCategoryForWrite(ctx, state, cats)
		if err != nil {
			return workflow.StepOutput[TransactionWriteState]{State: newState, Status: workflow.StepStatusFailed}, fmt.Errorf("workflows.transaction_write: validate_category: %w", err)
		}
		if override != nil {
			return *override, nil
		}
		if !ok {
			return workflow.StepOutput[TransactionWriteState]{State: newState, Status: workflow.StepStatusCompleted}, nil
		}
		state = newState
	}

	state.ResumeText = ""
	if idem != nil {
		return executeTransactionWithIdempotency(ctx, state, ledger, idem, metrics)
	}
	return executeTransactionDirectWrite(ctx, state, ledger, metrics)
}

func validateTransactionCategoryForWrite(ctx context.Context, state TransactionWriteState, cats categoryValidator) (TransactionWriteState, bool, *workflow.StepOutput[TransactionWriteState], error) {
	if len(state.Candidates) == 0 {
		out, _ := completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteFailure())
		return state, false, &out, nil
	}
	c := state.Candidates[0]
	if c.SubcategoryID == (uuid.UUID{}) || c.SubcategoryID == c.RootCategoryID {
		out, _ := completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteFailure())
		return state, false, &out, nil
	}
	if cats == nil {
		return state, true, nil, nil
	}
	if _, err := cats.ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
		RootCategoryID:  c.RootCategoryID,
		SubcategoryID:   c.SubcategoryID,
		Kind:            state.Kind,
		ExpectedVersion: state.CategoryVersion,
	}); err != nil {
		if errors.Is(err, catusecases.ErrKindMismatch) {
			newState, ok, output := reclassifyTransactionByKind(ctx, state, cats)
			return newState, ok, output, nil
		}
		if isCategoryBusinessRejection(err) {
			out, _ := completeTransaction(state, TransactionWriteStatusCancelled, messages.WriteFailure())
			return state, false, &out, nil
		}
		out, _ := completeTransaction(state, TransactionWriteStatusActive, messages.WriteFailure())
		return state, false, &out, err
	}
	return state, true, nil, nil
}

func reclassifyTransactionByKind(ctx context.Context, state TransactionWriteState, cats categoryValidator) (TransactionWriteState, bool, *workflow.StepOutput[TransactionWriteState]) {
	candidates, searchErr := SearchAndEnrichCandidates(ctx, cats, state.Description, state.Kind, state.CategoryVersion)
	if searchErr == nil && len(candidates) > 0 {
		state.Candidates = []PendingCategoryCandidate{candidates[0]}
		return state, true, nil
	}

	state.Awaiting = TransactionAwaitingCategory
	state.Candidates = nil
	output, _ := transactionCategoryReprompt(state, "Não encontrei uma categoria compatível para esse tipo de lançamento. Qual é a categoria?")
	return state, false, &output
}

func executeTransactionWithIdempotency(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger, idem IdempotentWriter, metrics *transactionWriteMetrics) (workflow.StepOutput[TransactionWriteState], error) {
	writeFn := IdempotentWriteFn(func(c context.Context) (uuid.UUID, bool, error) {
		ref, err := callTransactionLedger(c, state, ledger)
		if err != nil {
			return uuid.Nil, false, err
		}
		return ref.ID, ref.Reconciled, nil
	})

	classifier := DomainErrorClassifier(func(err error) bool {
		return errors.Is(err, txusecases.ErrPaymentMethodMigrationNotAllowed)
	})

	var (
		resourceID uuid.UUID
		outcome    agent.ToolOutcome
		idemErr    error
	)
	for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
		resourceID, outcome, idemErr = idem.Execute(ctx, state.UserID, state.MessageID, state.ItemSeq, state.OperationKind.String(), transactionResourceKind(state), writeFn, classifier)
		if idemErr == nil || !IsTransient(idemErr) || attempt == maxWriteAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return completeTransaction(state, TransactionWriteStatusActive, messages.WriteFailure())
		case <-time.After(backoffWithJitter(attempt)):
		}
	}
	if idemErr != nil {
		if errors.Is(idemErr, txusecases.ErrPaymentMethodMigrationNotAllowed) {
			return completeTransaction(state, TransactionWriteStatusCancelled, messages.PaymentMethodMigrationBlocked())
		}
		out, _ := completeTransaction(state, TransactionWriteStatusActive, messages.WriteFailure())
		return out, fmt.Errorf("workflows.transaction_write: idempotent_write: %w", idemErr)
	}
	pendingStatus, stepStatus, postErr := DecideTransactionPostWrite(outcome, resourceID)
	if postErr != nil {
		recordTransactionFalseSuccessIfNeeded(ctx, metrics, postErr)
		out, _ := completeTransaction(state, pendingStatus, messages.WriteFailure())
		out.Status = stepStatus
		return out, fmt.Errorf("workflows.transaction_write: idempotent_write: %w", postErr)
	}
	state.Status = pendingStatus
	state.ResourceID = resourceID
	state.ResponseText = buildTransactionWriteSuccessText(state)
	return workflow.StepOutput[TransactionWriteState]{State: state, Status: stepStatus}, nil
}

func executeTransactionDirectWrite(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger, metrics *transactionWriteMetrics) (workflow.StepOutput[TransactionWriteState], error) {
	var (
		ref      interfaces.EntryRef
		writeErr error
	)
	for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
		ref, writeErr = callTransactionLedger(ctx, state, ledger)
		if writeErr == nil || !IsTransient(writeErr) || attempt == maxWriteAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return completeTransaction(state, TransactionWriteStatusActive, messages.WriteFailure())
		case <-time.After(backoffWithJitter(attempt)):
		}
	}
	if writeErr != nil {
		if errors.Is(writeErr, txusecases.ErrPaymentMethodMigrationNotAllowed) {
			return completeTransaction(state, TransactionWriteStatusCancelled, messages.PaymentMethodMigrationBlocked())
		}
		out, _ := completeTransaction(state, TransactionWriteStatusActive, messages.WriteFailure())
		return out, fmt.Errorf("workflows.transaction_write: direct_write: %w", writeErr)
	}
	pendingStatus, stepStatus, postErr := DecideTransactionPostWrite(agent.ToolOutcomeRouted, ref.ID)
	if postErr != nil {
		recordTransactionFalseSuccessIfNeeded(ctx, metrics, postErr)
		out, _ := completeTransaction(state, pendingStatus, messages.WriteFailure())
		out.Status = stepStatus
		return out, fmt.Errorf("workflows.transaction_write: direct_write: %w", postErr)
	}
	state.Status = pendingStatus
	state.ResourceID = ref.ID
	state.ResponseText = buildTransactionWriteSuccessText(state)
	return workflow.StepOutput[TransactionWriteState]{State: state, Status: stepStatus}, nil
}

func recordTransactionFalseSuccessIfNeeded(ctx context.Context, metrics *transactionWriteMetrics, postErr error) {
	if metrics == nil || metrics.falseSuccess == nil {
		return
	}
	if errors.Is(postErr, ErrTransactionWriteAcceptedWithoutResource) {
		metrics.falseSuccess.Increment(ctx,
			observability.String("workflow", TransactionWriteWorkflowID),
			observability.String("step", stepTransactionWriteID),
		)
	}
}

type ledgerWriteFn func(context.Context, TransactionWriteState, interfaces.TransactionsLedger) (interfaces.EntryRef, error)

var transactionLedgerWriters = map[TransactionOperationKind]ledgerWriteFn{
	TransactionOpRegisterExpense:  writeCreateTransaction,
	TransactionOpRegisterIncome:   writeCreateTransaction,
	TransactionOpEditEntry:        writeUpdateTransaction,
	TransactionOpCreateRecurrence: writeCreateRecurringTemplate,
}

func callTransactionLedger(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger) (interfaces.EntryRef, error) {
	fn, ok := transactionLedgerWriters[state.OperationKind]
	if !ok {
		return interfaces.EntryRef{}, fmt.Errorf("workflows.transaction_write: operação sem escritor de ledger: %s", state.OperationKind)
	}
	return fn(ctx, state, ledger)
}

func writeCreateTransaction(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger) (interfaces.EntryRef, error) {
	return ledger.CreateTransaction(ctx, buildRawTransactionForWrite(state))
}

func writeUpdateTransaction(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger) (interfaces.EntryRef, error) {
	return ledger.UpdateTransaction(ctx, buildRawUpdateForWrite(state))
}

func writeCreateRecurringTemplate(ctx context.Context, state TransactionWriteState, ledger interfaces.TransactionsLedger) (interfaces.EntryRef, error) {
	return ledger.CreateRecurringTemplate(ctx, buildRawRecurringForWrite(state))
}

func transactionResourceKind(state TransactionWriteState) string {
	if state.OperationKind == TransactionOpCreateRecurrence {
		return "recurring_template"
	}
	return "transaction"
}

func transactionDirection(kind interfaces.CategoryKind) string {
	if kind == interfaces.CategoryKindIncome {
		return "income"
	}
	return "outcome"
}

func chosenTransactionCategory(state TransactionWriteState) (uuid.UUID, *uuid.UUID) {
	if len(state.Candidates) > 0 {
		c := state.Candidates[0]
		var sub *uuid.UUID
		if c.SubcategoryID != (uuid.UUID{}) {
			s := c.SubcategoryID
			sub = &s
		}
		return c.RootCategoryID, sub
	}
	return state.TargetCategoryID, state.TargetSubcategoryID
}

type chosenTransactionEvidence struct {
	outcome     string
	score       float64
	confidence  string
	quality     string
	signalType  string
	matchedTerm string
	matchReason string
}

const transactionCategoryOutcomeMatched = "matched"

func chosenTransactionEvidenceFor(state TransactionWriteState) chosenTransactionEvidence {
	if len(state.Candidates) == 0 {
		return chosenTransactionEvidence{}
	}
	c := state.Candidates[0]
	return chosenTransactionEvidence{
		outcome:     transactionCategoryOutcomeMatched,
		score:       c.Score,
		confidence:  c.Confidence,
		quality:     c.MatchQuality,
		signalType:  c.SignalType,
		matchedTerm: c.MatchedTerm,
		matchReason: c.MatchReason,
	}
}

func buildRawTransactionForWrite(state TransactionWriteState) interfaces.RawTransaction {
	catID, sub := chosenTransactionCategory(state)
	ev := chosenTransactionEvidenceFor(state)
	return interfaces.RawTransaction{
		Direction:           transactionDirection(state.Kind),
		PaymentMethod:       state.PaymentMethod,
		AmountCents:         state.AmountCents,
		Description:         state.Description,
		CategoryID:          catID,
		SubcategoryID:       sub,
		CardID:              state.CardID,
		Installments:        state.Installments,
		OccurredAt:          state.OccurredAt,
		OriginWamid:         state.MessageID,
		OriginItemSeq:       state.ItemSeq,
		OriginOperation:     originOperationTransactionWrite,
		CategorySource:      transactionCategorySrcUser,
		CategoryOutcome:     ev.outcome,
		CategoryScore:       ev.score,
		CategoryConfidence:  ev.confidence,
		CategoryQuality:     ev.quality,
		CategorySignalType:  ev.signalType,
		CategoryMatchedTerm: ev.matchedTerm,
		CategoryMatchReason: ev.matchReason,
		CategoryVersion:     state.CategoryVersion,
	}
}

func buildRawUpdateForWrite(state TransactionWriteState) interfaces.RawUpdateTransaction {
	catID, sub := chosenTransactionCategory(state)
	ev := chosenTransactionEvidenceFor(state)
	txID := uuid.UUID{}
	if state.TargetTransactionID != nil {
		txID = *state.TargetTransactionID
	}
	paymentMethod := state.TargetPaymentMethod
	if state.PaymentMethod != "" {
		paymentMethod = state.PaymentMethod
	}
	return interfaces.RawUpdateTransaction{
		ID:                  txID,
		Direction:           transactionDirection(state.Kind),
		PaymentMethod:       paymentMethod,
		AmountCents:         state.AmountCents,
		Description:         state.Description,
		CategoryID:          catID,
		SubcategoryID:       sub,
		OccurredAt:          state.OccurredAt,
		Version:             state.TargetVersion,
		CategorySource:      transactionCategorySrcUser,
		CategoryOutcome:     ev.outcome,
		CategoryScore:       ev.score,
		CategoryConfidence:  ev.confidence,
		CategoryQuality:     ev.quality,
		CategorySignalType:  ev.signalType,
		CategoryMatchedTerm: ev.matchedTerm,
		CategoryMatchReason: ev.matchReason,
		CategoryVersion:     state.CategoryVersion,
	}
}

func buildRawRecurringForWrite(state TransactionWriteState) interfaces.RawRecurringTemplate {
	catID, sub := chosenTransactionCategory(state)
	startedAt := state.OccurredAt
	if startedAt == "" {
		startedAt = time.Now().UTC().Format("2006-01-02")
	}
	return interfaces.RawRecurringTemplate{
		Direction:       transactionDirection(state.Kind),
		PaymentMethod:   state.PaymentMethod,
		CardID:          state.CardID,
		AmountCents:     state.AmountCents,
		Description:     state.Description,
		CategoryID:      catID,
		SubcategoryID:   sub,
		Frequency:       state.Frequency,
		DayOfMonth:      state.RecurrenceDayOfMonth,
		StartedAt:       startedAt,
		OriginWamid:     state.MessageID,
		OriginOperation: originOperationTransactionWrite,
		CategorySource:  transactionCategorySrcUser,
		CategoryVersion: state.CategoryVersion,
	}
}

func categoryPathFor(state TransactionWriteState) string {
	if len(state.Candidates) > 0 {
		return state.Candidates[0].Path
	}
	return state.EditPreviousCategory
}

type successTextFn func(TransactionWriteState) string

var transactionSuccessTextByOperation = map[TransactionOperationKind]successTextFn{
	TransactionOpCreateRecurrence: successTextRecurrence,
	TransactionOpEditEntry:        successTextEdit,
	TransactionOpRegisterIncome:   successTextIncome,
	TransactionOpRegisterExpense:  successTextExpense,
}

func successTextRecurrence(state TransactionWriteState) string {
	return messages.RecurrenceSuccess(messages.NewMotivationSeed(state.MessageID))
}

func successTextEdit(state TransactionWriteState) string {
	return messages.EditSuccess(messages.NewMotivationSeed(state.MessageID))
}

func successTextIncome(state TransactionWriteState) string {
	return messages.WriteSuccess(messages.WriteKindIncome, messages.NewMotivationSeed(state.MessageID))
}

func successTextExpense(state TransactionWriteState) string {
	return messages.WriteSuccess(messages.WriteKindExpense, messages.NewMotivationSeed(state.MessageID))
}

func buildTransactionWriteSuccessText(state TransactionWriteState) string {
	fn, ok := transactionSuccessTextByOperation[state.OperationKind]
	if !ok {
		return successTextExpense(state)
	}
	return fn(state)
}

type confirmSummaryFn func(TransactionWriteState) string

var transactionConfirmSummaryByOperation = map[TransactionOperationKind]confirmSummaryFn{
	TransactionOpRegisterIncome:   confirmSummaryIncome,
	TransactionOpCreateRecurrence: confirmSummaryRecurrence,
	TransactionOpEditEntry:        confirmSummaryEdit,
	TransactionOpRegisterExpense:  confirmSummaryExpense,
}

func confirmSummaryIncome(state TransactionWriteState) string {
	return messages.IncomeConfirmationBlock(messages.ConfirmationView{
		AmountFormatted: money.FromCents(state.AmountCents).BRL(),
		Origin:          state.Description,
	})
}

func confirmSummaryRecurrence(state TransactionWriteState) string {
	return messages.RecurrenceConfirmationBlock(messages.RecurrenceConfirmationView{
		AmountFormatted: money.FromCents(state.AmountCents).BRL(),
		Category:        categoryPathFor(state),
		Frequency:       state.Frequency,
	})
}

func confirmSummaryEdit(state TransactionWriteState) string {
	newPayment := formatPaymentLabel(state.PaymentMethod)
	if newPayment == "" {
		newPayment = state.EditPreviousPayment
	}
	newCategory := categoryPathFor(state)
	return messages.EditConfirmationBlock(messages.EditConfirmationView{
		PreviousAmountFormatted: money.FromCents(state.EditPreviousAmountCents).BRL(),
		PreviousCategory:        state.EditPreviousCategory,
		PreviousPaymentMethod:   state.EditPreviousPayment,
		NewAmountFormatted:      money.FromCents(state.AmountCents).BRL(),
		NewCategory:             newCategory,
		NewPaymentMethod:        newPayment,
		AmountChanged:           state.AmountCents != state.EditPreviousAmountCents,
		CategoryChanged:         newCategory != state.EditPreviousCategory,
		PaymentChanged:          state.EditPaymentMethodChanged,
	})
}

func confirmSummaryExpense(state TransactionWriteState) string {
	return messages.ExpenseConfirmationBlock(messages.ConfirmationView{
		AmountFormatted: money.FromCents(state.AmountCents).BRL(),
		PaymentMethod:   formatPaymentLabel(state.PaymentMethod),
		Category:        categoryPathFor(state),
	})
}

func buildTransactionConfirmSummary(state TransactionWriteState) string {
	fn, ok := transactionConfirmSummaryByOperation[state.OperationKind]
	if !ok {
		return confirmSummaryExpense(state)
	}
	return fn(state)
}

func buildTransactionSlotPrompt(state TransactionWriteState) string {
	switch state.Awaiting {
	case TransactionAwaitingCategory:
		if len(state.Candidates) > 1 {
			return buildCandidatesPrompt(state.Candidates)
		}
		return messages.ClarificationQuestion(messages.MissingFieldCategory)
	case TransactionAwaitingPaymentMethod:
		return messages.ClarificationQuestion(messages.MissingFieldPaymentMethod)
	case TransactionAwaitingCard:
		return messages.CardPrompt()
	case TransactionAwaitingDate:
		return messages.DatePrompt()
	case TransactionAwaitingEditCandidate:
		return buildEditCandidatesPrompt(state.EditCandidates)
	case TransactionAwaitingConfirmation:
		return buildTransactionConfirmSummary(state)
	default:
		return messages.ClarificationQuestion(messages.MissingFieldUnspecified)
	}
}

func buildTransactionSlotReprompt(state TransactionWriteState) string {
	switch state.Awaiting {
	case TransactionAwaitingCategory:
		if len(state.Candidates) > 1 {
			return buildCandidatesPrompt(state.Candidates)
		}
		return messages.ClarificationQuestion(messages.MissingFieldCategory)
	case TransactionAwaitingPaymentMethod:
		return messages.ClarificationQuestion(messages.MissingFieldPaymentMethod)
	case TransactionAwaitingCard:
		return messages.CardPrompt()
	case TransactionAwaitingDate:
		return messages.DatePrompt()
	default:
		return buildTransactionSlotPrompt(state)
	}
}

func ContinueTransactionWrite(
	ctx context.Context,
	engine workflow.Engine[TransactionWriteState],
	def workflow.Definition[TransactionWriteState],
	key string,
	userMessage string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage})
	if err != nil {
		return false, "", fmt.Errorf("workflows.transaction_write: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.transaction_write: resume: %w", resumeErr)
	}

	return true, result.State.ResponseText, nil
}
