package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type IdempotentWriteFn func(ctx context.Context) (resourceID uuid.UUID, reconciled bool, err error)

type IdempotentWriter interface {
	Execute(
		ctx context.Context,
		userID uuid.UUID,
		wamid string,
		itemSeq int,
		operation string,
		resourceKind string,
		write IdempotentWriteFn,
	) (resourceID uuid.UUID, outcome agent.ToolOutcome, err error)
}

func resourceKindForState(state PendingEntryState) string {
	if state.OperationKind == PendingOpCreateRecurrence {
		return "recurring_template"
	}
	return "transaction"
}

type cardNicknameSolver interface {
	ResolveCardByNickname(ctx context.Context, userID uuid.UUID, nickname string) (interfaces.Card, error)
}

type categoryValidator interface {
	SearchDictionary(ctx context.Context, term, kind string) (interfaces.CategorySearchResult, error)
	ResolveForWrite(ctx context.Context, input interfaces.CategoryWriteRequest) (interfaces.CategoryWriteDecision, error)
}

const (
	PendingEntryWorkflowID  = "pending-entry"
	stepPendingEntryID      = "pending-entry"
	PendingEntryStaleAfter  = 35 * time.Minute
	PendingEntryReaperBatch = 100
	originOperationPending  = "pending_entry_register"
	categorySrcUserSelected = "user_selected_candidate"
)

type PendingEntryMode int

const (
	PendingEntryModeReplied PendingEntryMode = iota + 1
	PendingEntryModePassThrough
	PendingEntryModeCompleted
	PendingEntryModeCancelled
	PendingEntryModeExpired
	PendingEntryModeReplaced
)

type PendingEntryResult struct {
	Handled bool
	Message string
	Mode    PendingEntryMode
}

func PendingEntryKey(resourceID, threadID string) string {
	return resourceID + ":" + threadID + ":" + PendingEntryWorkflowID
}

func BuildPendingEntryWorkflow(ledger interfaces.TransactionsLedger, cards cardNicknameSolver, cats categoryValidator, idem IdempotentWriter) workflow.Definition[PendingEntryState] {
	step := workflow.NewStepFunc(stepPendingEntryID, makePendingEntryStep(ledger, cards, cats, idem))
	return workflow.Definition[PendingEntryState]{
		ID:          PendingEntryWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildPendingEntryReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, PendingEntryWorkflowID, PendingEntryStaleAfter, PendingEntryReaperBatch, o11y)
}

func makePendingEntryStep(ledger interfaces.TransactionsLedger, cards cardNicknameSolver, cats categoryValidator, idem IdempotentWriter) func(context.Context, PendingEntryState) (workflow.StepOutput[PendingEntryState], error) {
	return func(ctx context.Context, state PendingEntryState) (workflow.StepOutput[PendingEntryState], error) {
		if state.ResumeText == "" {
			state.SuspendedAt = time.Now().UTC()
			state.ResponseText = buildSlotPrompt(state)
			return workflow.StepOutput[PendingEntryState]{
				State:  state,
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: state.ResponseText,
				},
			}, nil
		}

		msg := PendingMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
		now := time.Now().UTC()

		switch state.Awaiting {
		case AwaitingSlotCategory:
			return handleCategorySlotResume(ctx, state, msg, now, cats)
		case AwaitingSlotCard:
			return handleCardSlotResume(ctx, state, msg, now, cards)
		case AwaitingSlotConfirmation:
			return handleConfirmationResume(ctx, state, msg, now, ledger, cats, idem)
		default:
			return handleSlotResume(state, msg, now)
		}
	}
}

func handleCardSlotResume(ctx context.Context, state PendingEntryState, msg PendingMessage, now time.Time, cards cardNicknameSolver) (workflow.StepOutput[PendingEntryState], error) {
	if isExpired(state, now) {
		state.Status = PendingStatusExpired
		state.ResponseText = "O registro expirou. Para registrar, envie a informação completa novamente."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	text := strings.TrimSpace(msg.Text)

	if isCancelMessage(text) {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Tudo certo, o registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	if isNewCompleteOperation(text) {
		state.Status = PendingStatusReplaced
		state.ResponseText = ""
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	if cards != nil {
		card, err := cards.ResolveCardByNickname(ctx, state.UserID, text)
		if err == nil {
			cardUUID, parseErr := uuid.Parse(card.ID)
			if parseErr == nil {
				state.CardID = &cardUUID
				state.Awaiting = AwaitingSlotConfirmation
				state.SuspendedAt = time.Now().UTC()
				state.ResumeText = ""
				state.RepromptCount = 0
				state.ResponseText = buildConfirmSummary(state)
				return workflow.StepOutput[PendingEntryState]{
					State:  state,
					Status: workflow.StepStatusSuspended,
					Suspend: &workflow.Suspension{
						Reason: workflow.SuspendAwaitingInput,
						Prompt: state.ResponseText,
					},
				}, nil
			}
		}
	}

	if state.RepromptCount >= maxReprompts {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui identificar o cartão. O registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	state.RepromptCount++
	state.ResumeText = ""
	state.ResponseText = buildSlotReprompt(state)
	return workflow.StepOutput[PendingEntryState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: state.ResponseText,
		},
	}, nil
}

func handleCategorySlotResume(ctx context.Context, state PendingEntryState, msg PendingMessage, now time.Time, cats categoryValidator) (workflow.StepOutput[PendingEntryState], error) {
	if isExpired(state, now) {
		state.Status = PendingStatusExpired
		state.ResponseText = "O registro expirou. Para registrar, envie a informação completa novamente."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	text := strings.TrimSpace(msg.Text)

	if isCancelMessage(text) {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Tudo certo, o registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	if isNewCompleteOperation(text) {
		state.Status = PendingStatusReplaced
		state.ResponseText = ""
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	if len(state.Candidates) > 0 {
		decision, decErr := DecideCategoryChoice(state, state.Candidates, text)
		if decErr == nil && decision.Action == CategoryChoiceActionSelected {
			return promoteCategoryToConfirmation(state, decision.Candidate)
		}
		if decErr == nil && decision.Action == CategoryChoiceActionRootOnly {
			return categorySlotReprompt(state, "Essa categoria precisa de uma subcategoria específica. Qual você quer usar?")
		}
	}

	if cats != nil {
		candidates, searchErr := SearchAndEnrichCandidates(ctx, cats, text, state.Kind, state.CategoryVersion)
		if searchErr == nil && len(candidates) == 1 {
			return promoteCategoryToConfirmation(state, candidates[0])
		}
		if searchErr == nil && len(candidates) > 1 {
			state.Candidates = candidates
			state.Awaiting = AwaitingSlotCategory
			state.SuspendedAt = time.Now().UTC()
			state.ResumeText = ""
			state.RepromptCount = 0
			state.ResponseText = buildCandidatesPrompt(candidates)
			return workflow.StepOutput[PendingEntryState]{
				State:  state,
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: state.ResponseText,
				},
			}, nil
		}
	}

	return categorySlotReprompt(state, buildSlotReprompt(state))
}

func promoteCategoryToConfirmation(state PendingEntryState, candidate PendingCategoryCandidate) (workflow.StepOutput[PendingEntryState], error) {
	state.Candidates = []PendingCategoryCandidate{candidate}
	state.Awaiting = AwaitingSlotConfirmation
	state.SuspendedAt = time.Now().UTC()
	state.ResumeText = ""
	state.RepromptCount = 0
	state.ResponseText = buildConfirmSummary(state)
	return workflow.StepOutput[PendingEntryState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: state.ResponseText,
		},
	}, nil
}

func categorySlotReprompt(state PendingEntryState, prompt string) (workflow.StepOutput[PendingEntryState], error) {
	if state.RepromptCount >= maxReprompts {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui identificar a categoria. O registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
	state.RepromptCount++
	state.ResumeText = ""
	state.ResponseText = prompt
	return workflow.StepOutput[PendingEntryState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: state.ResponseText,
		},
	}, nil
}

func handleSlotResume(state PendingEntryState, msg PendingMessage, now time.Time) (workflow.StepOutput[PendingEntryState], error) {
	decision, err := DecidePendingResume(state, msg, now)
	if err != nil {
		return workflow.StepOutput[PendingEntryState]{}, fmt.Errorf("workflows.pending_entry: decide resume: %w", err)
	}

	switch decision.Action {
	case PendingActionExpire:
		state.Status = PendingStatusExpired
		state.ResponseText = "O registro expirou. Para registrar, envie a informação completa novamente."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case PendingActionCancel:
		state.Status = PendingStatusCancelled
		state.ResponseText = "Tudo certo, o registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case PendingActionReplace:
		state.Status = PendingStatusReplaced
		state.ResponseText = ""
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case PendingActionReprompt:
		state.RepromptCount++
		state.ResumeText = ""
		state.ResponseText = buildSlotReprompt(state)
		return workflow.StepOutput[PendingEntryState]{
			State:  state,
			Status: workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{
				Reason: workflow.SuspendAwaitingInput,
				Prompt: state.ResponseText,
			},
		}, nil

	case PendingActionFillSlot:
		if decision.SlotFilled == AwaitingSlotPaymentMethod && decision.FilledValue != "" {
			state.PaymentMethod = decision.FilledValue
		}
		if decision.SlotFilled == AwaitingSlotDate && decision.FilledValue != "" {
			state.OccurredAt = decision.FilledValue
		}
		state.Awaiting = AwaitingSlotConfirmation
		state.SuspendedAt = time.Now().UTC()
		state.ResumeText = ""
		state.RepromptCount = 0
		state.ResponseText = buildConfirmSummary(state)
		return workflow.StepOutput[PendingEntryState]{
			State:  state,
			Status: workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{
				Reason: workflow.SuspendAwaitingInput,
				Prompt: state.ResponseText,
			},
		}, nil

	default:
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não foi possível processar a resposta. O registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func handleConfirmationResume(ctx context.Context, state PendingEntryState, msg PendingMessage, now time.Time, ledger interfaces.TransactionsLedger, cats categoryValidator, idem IdempotentWriter) (workflow.StepOutput[PendingEntryState], error) {
	decision, err := DecideConfirmation(state, msg, now)
	if err != nil {
		return workflow.StepOutput[PendingEntryState]{}, fmt.Errorf("workflows.pending_entry: decide confirmation: %w", err)
	}

	switch decision.Action {
	case ConfirmActionAccept:
		return executeWrite(ctx, state, ledger, cats, idem)

	case ConfirmActionCancel:
		state.Status = PendingStatusCancelled
		state.ResponseText = "Tudo certo, o registro foi cancelado."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case ConfirmActionExpire:
		state.Status = PendingStatusExpired
		state.ResponseText = "O registro expirou. Para registrar, envie a informação completa novamente."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case ConfirmActionReplay:
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil

	case ConfirmActionReprompt:
		state.ConfirmRepromptCount++
		state.ProcessedMessageID = msg.MessageID
		state.ResumeText = ""
		state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar o registro."
		return workflow.StepOutput[PendingEntryState]{
			State:  state,
			Status: workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{
				Reason: workflow.SuspendAwaitingInput,
				Prompt: state.ResponseText,
			},
		}, nil

	default:
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não foi possível confirmar o registro."
		state.ResumeText = ""
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func executeWrite(ctx context.Context, state PendingEntryState, ledger interfaces.TransactionsLedger, cats categoryValidator, idem IdempotentWriter) (workflow.StepOutput[PendingEntryState], error) {
	state, ok := validateCategoryForWrite(ctx, state, cats)
	if !ok {
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
	state.ResumeText = ""
	if idem != nil {
		return executeWithIdempotency(ctx, state, ledger, idem)
	}
	return executeDirectWrite(ctx, state, ledger)
}

func validateCategoryForWrite(ctx context.Context, state PendingEntryState, cats categoryValidator) (PendingEntryState, bool) {
	if len(state.Candidates) == 0 {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui validar a categoria. O registro foi cancelado."
		state.ResumeText = ""
		return state, false
	}
	c := state.Candidates[0]
	if c.SubcategoryID == (uuid.UUID{}) || c.SubcategoryID == c.RootCategoryID {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui validar a categoria. O registro foi cancelado."
		state.ResumeText = ""
		return state, false
	}
	if cats == nil {
		return state, true
	}
	if _, err := cats.ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
		RootCategoryID:  c.RootCategoryID,
		SubcategoryID:   c.SubcategoryID,
		Kind:            state.Kind,
		ExpectedVersion: state.CategoryVersion,
	}); err != nil {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui validar a categoria. O registro foi cancelado."
		state.ResumeText = ""
		return state, false
	}
	return state, true
}

func executeWithIdempotency(ctx context.Context, state PendingEntryState, ledger interfaces.TransactionsLedger, idem IdempotentWriter) (workflow.StepOutput[PendingEntryState], error) {
	writeFn := IdempotentWriteFn(func(c context.Context) (uuid.UUID, bool, error) {
		ref, err := callLedger(c, state, ledger)
		if err != nil {
			return uuid.Nil, false, err
		}
		return ref.ID, ref.Reconciled, nil
	})
	resourceID, outcome, idemErr := idem.Execute(ctx, state.UserID, state.MessageID, state.ItemSeq, state.OperationKind.String(), resourceKindForState(state), writeFn)
	if idemErr != nil || (outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil) {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui registrar. Tente novamente em breve."
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
	state.Status = PendingStatusCompleted
	state.ResponseText = buildWriteSuccessText(state)
	return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func executeDirectWrite(ctx context.Context, state PendingEntryState, ledger interfaces.TransactionsLedger) (workflow.StepOutput[PendingEntryState], error) {
	ref, writeErr := callLedger(ctx, state, ledger)
	if writeErr != nil || ref.ID == uuid.Nil {
		state.Status = PendingStatusCancelled
		state.ResponseText = "Não consegui registrar. Tente novamente em breve."
		return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
	state.Status = PendingStatusCompleted
	state.ResponseText = buildWriteSuccessText(state)
	return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func callLedger(ctx context.Context, state PendingEntryState, ledger interfaces.TransactionsLedger) (interfaces.EntryRef, error) {
	switch state.OperationKind {
	case PendingOpEditEntry:
		return ledger.UpdateTransaction(ctx, buildRawUpdate(state))
	case PendingOpCreateRecurrence:
		return ledger.CreateRecurringTemplate(ctx, buildRawRecurring(state))
	default:
		return ledger.CreateTransaction(ctx, buildRawTransaction(state))
	}
}

func pendingDirection(catKind interfaces.CategoryKind) string {
	if catKind == interfaces.CategoryKindIncome {
		return "income"
	}
	return "outcome"
}

func chosenCandidate(state PendingEntryState) (catID uuid.UUID, sub *uuid.UUID) {
	if len(state.Candidates) == 0 {
		return uuid.UUID{}, nil
	}
	c := state.Candidates[0]
	catID = c.RootCategoryID
	if c.SubcategoryID != (uuid.UUID{}) {
		s := c.SubcategoryID
		sub = &s
	}
	return catID, sub
}

func buildRawTransaction(state PendingEntryState) interfaces.RawTransaction {
	catID, sub := chosenCandidate(state)
	return interfaces.RawTransaction{
		Direction:       pendingDirection(state.Kind),
		PaymentMethod:   state.PaymentMethod,
		AmountCents:     state.AmountCents,
		Description:     state.Description,
		CategoryID:      catID,
		SubcategoryID:   sub,
		CardID:          state.CardID,
		Installments:    state.Installments,
		OccurredAt:      state.OccurredAt,
		OriginWamid:     state.MessageID,
		OriginOperation: originOperationPending,
		CategorySource:  categorySrcUserSelected,
		CategoryVersion: state.CategoryVersion,
	}
}

func buildRawUpdate(state PendingEntryState) interfaces.RawUpdateTransaction {
	catID, sub := chosenCandidate(state)
	txID := uuid.UUID{}
	if state.TargetTransactionID != nil {
		txID = *state.TargetTransactionID
	}
	return interfaces.RawUpdateTransaction{
		ID:              txID,
		Direction:       pendingDirection(state.Kind),
		PaymentMethod:   state.PaymentMethod,
		AmountCents:     state.AmountCents,
		Description:     state.Description,
		CategoryID:      catID,
		SubcategoryID:   sub,
		OccurredAt:      state.OccurredAt,
		Version:         state.TargetVersion,
		CategorySource:  categorySrcUserSelected,
		CategoryVersion: state.CategoryVersion,
	}
}

func buildRawRecurring(state PendingEntryState) interfaces.RawRecurringTemplate {
	catID, sub := chosenCandidate(state)
	startedAt := state.OccurredAt
	if startedAt == "" {
		startedAt = time.Now().UTC().Format("2006-01-02")
	}
	return interfaces.RawRecurringTemplate{
		Direction:       pendingDirection(state.Kind),
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
		OriginOperation: originOperationPending,
		CategorySource:  categorySrcUserSelected,
		CategoryVersion: state.CategoryVersion,
	}
}

func formatAmountBR(cents int64) string {
	intPart := cents / 100
	fracPart := cents % 100
	if fracPart < 0 {
		fracPart = -fracPart
	}
	return fmt.Sprintf("R$ %d,%02d", intPart, fracPart)
}

func formatDateLabel(raw string) string {
	if raw == "" {
		return ""
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return ""
	}
	loc, locErr := time.LoadLocation("America/Sao_Paulo")
	if locErr != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	dLocal := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	formatted := dLocal.Format("02/01/2006")
	switch {
	case dLocal.Equal(today):
		return fmt.Sprintf("hoje (%s)", formatted)
	case dLocal.Equal(today.Add(-24 * time.Hour)):
		return fmt.Sprintf("ontem (%s)", formatted)
	default:
		return formatted
	}
}

func formatPaymentLabel(method string) string {
	switch method {
	case "pix":
		return "pix"
	case "debit_card":
		return "débito"
	case "debit_in_account":
		return "débito em conta"
	case "cash":
		return "dinheiro"
	case "boleto":
		return "boleto"
	case "credit_card":
		return "crédito"
	case "vale_refeicao":
		return "vale-refeição"
	case "vale_alimentacao":
		return "vale-alimentação"
	case "ted":
		return "TED"
	default:
		return method
	}
}

func buildCandidatesPrompt(candidates []PendingCategoryCandidate) string {
	var parts []string
	for i, c := range candidates {
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, c.Path))
	}
	return "Qual se encaixa melhor? " + strings.Join(parts, " ")
}

func buildWriteSuccessText(state PendingEntryState) string {
	amountStr := formatAmountBR(state.AmountCents)
	categoryPath := ""
	if len(state.Candidates) > 0 {
		categoryPath = state.Candidates[0].Path
	}
	dateLabel := formatDateLabel(state.OccurredAt)
	paymentLabel := formatPaymentLabel(state.PaymentMethod)
	switch state.OperationKind {
	case PendingOpCreateRecurrence:
		if categoryPath != "" {
			return fmt.Sprintf("Recorrência de %s em *%s* configurada com sucesso.", amountStr, categoryPath)
		}
		return fmt.Sprintf("Recorrência de %s configurada com sucesso.", amountStr)
	case PendingOpEditEntry:
		if categoryPath != "" {
			return fmt.Sprintf("Lançamento atualizado para %s em *%s*.", amountStr, categoryPath)
		}
		return fmt.Sprintf("Lançamento atualizado para %s.", amountStr)
	default:
		prefix := "Despesa"
		if state.Kind == interfaces.CategoryKindIncome {
			prefix = "Receita"
		}
		if categoryPath != "" && dateLabel != "" {
			return fmt.Sprintf("%s de %s registrada em *%s* para %s no %s ✅", prefix, amountStr, categoryPath, dateLabel, paymentLabel)
		}
		if categoryPath != "" {
			return fmt.Sprintf("%s de %s registrada em *%s* ✅", prefix, amountStr, categoryPath)
		}
		return fmt.Sprintf("%s de %s registrada com sucesso ✅", prefix, amountStr)
	}
}

func buildSlotPrompt(state PendingEntryState) string {
	switch state.Awaiting {
	case AwaitingSlotCategory:
		if len(state.Candidates) > 1 {
			return buildCandidatesPrompt(state.Candidates)
		}
		return "Qual é a categoria deste lançamento?"
	case AwaitingSlotPaymentMethod:
		return "Qual foi a forma de pagamento?"
	case AwaitingSlotCard:
		return "Qual cartão foi utilizado?"
	case AwaitingSlotDate:
		return "Qual foi a data do lançamento?"
	case AwaitingSlotConfirmation:
		return buildConfirmSummary(state)
	case AwaitingSlotCorrection:
		return "O que deseja corrigir no lançamento?"
	default:
		return "Por favor, informe os dados solicitados."
	}
}

func buildSlotReprompt(state PendingEntryState) string {
	switch state.Awaiting {
	case AwaitingSlotCategory:
		if len(state.Candidates) > 1 {
			return buildCandidatesPrompt(state.Candidates)
		}
		return "Não reconheci a categoria. Qual é a categoria deste lançamento?"
	case AwaitingSlotPaymentMethod:
		return "Não reconheci a forma de pagamento. Qual foi (pix, débito, crédito, dinheiro, boleto)?"
	case AwaitingSlotCard:
		return "Não reconheci o cartão. Qual cartão foi utilizado?"
	case AwaitingSlotDate:
		return "Não reconheci a data. Qual foi a data do lançamento?"
	default:
		return buildSlotPrompt(state)
	}
}

func buildConfirmSummary(state PendingEntryState) string {
	categoryPath := ""
	if len(state.Candidates) > 0 {
		categoryPath = state.Candidates[0].Path
	}
	parts := make([]string, 0, 5)
	if state.Description != "" {
		parts = append(parts, fmt.Sprintf("*%s*", state.Description))
	}
	parts = append(parts, formatAmountBR(state.AmountCents))
	if categoryPath != "" {
		parts = append(parts, fmt.Sprintf("em *%s*", categoryPath))
	}
	if dateLabel := formatDateLabel(state.OccurredAt); dateLabel != "" {
		parts = append(parts, fmt.Sprintf("para %s", dateLabel))
	}
	if payment := confirmPaymentSegment(state); payment != "" {
		parts = append(parts, payment)
	}
	return fmt.Sprintf("Confirma? %s?", strings.Join(parts, " "))
}

func confirmPaymentSegment(state PendingEntryState) string {
	if state.Kind == interfaces.CategoryKindIncome {
		return ""
	}
	if state.PaymentMethod == "credit_card" {
		if state.Installments > 1 {
			return fmt.Sprintf("no crédito em %dx", state.Installments)
		}
		return "no crédito à vista"
	}
	label := formatPaymentLabel(state.PaymentMethod)
	if label == "" {
		return ""
	}
	return fmt.Sprintf("no %s", label)
}
