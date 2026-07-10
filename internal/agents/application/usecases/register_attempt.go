package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type RegisterAttempt struct {
	categories interfaces.CategoriesReader
	ledger     interfaces.TransactionsLedger
	engine     wf.Engine[workflows.PendingEntryState]
	def        wf.Definition[workflows.PendingEntryState]
	o11y       observability.Observability
}

func NewRegisterAttempt(
	categories interfaces.CategoriesReader,
	ledger interfaces.TransactionsLedger,
	engine wf.Engine[workflows.PendingEntryState],
	def wf.Definition[workflows.PendingEntryState],
	o11y observability.Observability,
) *RegisterAttempt {
	return &RegisterAttempt{
		categories: categories,
		ledger:     ledger,
		engine:     engine,
		def:        def,
		o11y:       o11y,
	}
}

type CreateRecurrenceCommand struct {
	UserID          uuid.UUID
	ThreadID        string
	WAMID           string
	ItemSeq         int
	Direction       string
	PaymentMethod   string
	CardID          *uuid.UUID
	AmountCents     int64
	Description     string
	CategoryID      uuid.UUID
	SubcategoryID   uuid.UUID
	CategoryVersion int64
	Frequency       string
	DayOfMonth      int
	StartedAt       string
}

type EditEntryCommand struct {
	UserID              uuid.UUID
	ThreadID            string
	WAMID               string
	ItemSeq             int
	TargetTransactionID uuid.UUID
	AmountCents         int64
	Description         string
	OccurredAt          string
}

func (uc *RegisterAttempt) RegisterExpense(ctx context.Context, cmd RegisterExpenseCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_attempt.expense")
	defer span.End()

	awaiting, candidates, catVersion, classifyErr := uc.classifyForPending(ctx, cmd.Description, interfaces.CategoryKindExpense, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.expense: classify: %w", classifyErr)
	}

	awaiting = workflows.DecideInitialAwaiting(awaiting, cmd.PaymentMethod, cmd.CardID != nil)

	state := workflows.PendingEntryState{
		Status:          workflows.PendingStatusActive,
		Awaiting:        awaiting,
		OperationKind:   workflows.PendingOpRegisterExpense,
		UserID:          cmd.UserID,
		ResourceID:      cmd.UserID,
		ThreadID:        cmd.ThreadID,
		MessageID:       cmd.WAMID,
		ItemSeq:         cmd.ItemSeq,
		AmountCents:     cmd.AmountCents,
		Description:     cmd.Description,
		PaymentMethod:   cmd.PaymentMethod,
		CardID:          cmd.CardID,
		Installments:    cmd.Installments,
		OccurredAt:      resolveEntryDate(cmd.OccurredAt),
		Kind:            interfaces.CategoryKindExpense,
		Candidates:      candidates,
		CategoryVersion: catVersion,
	}

	key := workflows.PendingEntryKey(cmd.UserID.String(), cmd.ThreadID)
	startResult, err := uc.engine.Start(ctx, uc.def, key, state)
	if err != nil {
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: workflows.ActivePendingEntryMessage}, nil
		}
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.expense: start workflow: %w", err)
	}

	return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: pendingPrompt(startResult)}, nil
}

func (uc *RegisterAttempt) RegisterIncome(ctx context.Context, cmd RegisterIncomeCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_attempt.income")
	defer span.End()

	awaiting, candidates, catVersion, classifyErr := uc.classifyForPending(ctx, cmd.Description, interfaces.CategoryKindIncome, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.income: classify: %w", classifyErr)
	}

	state := workflows.PendingEntryState{
		Status:          workflows.PendingStatusActive,
		Awaiting:        awaiting,
		OperationKind:   workflows.PendingOpRegisterIncome,
		UserID:          cmd.UserID,
		ResourceID:      cmd.UserID,
		ThreadID:        cmd.ThreadID,
		MessageID:       cmd.WAMID,
		ItemSeq:         cmd.ItemSeq,
		AmountCents:     cmd.AmountCents,
		Description:     cmd.Description,
		PaymentMethod:   registerIncomePaymentMethod,
		OccurredAt:      resolveEntryDate(cmd.OccurredAt),
		Kind:            interfaces.CategoryKindIncome,
		Candidates:      candidates,
		CategoryVersion: catVersion,
	}

	key := workflows.PendingEntryKey(cmd.UserID.String(), cmd.ThreadID)
	startResult, err := uc.engine.Start(ctx, uc.def, key, state)
	if err != nil {
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: workflows.ActivePendingEntryMessage}, nil
		}
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.income: start workflow: %w", err)
	}

	return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: pendingPrompt(startResult)}, nil
}

func (uc *RegisterAttempt) CreateRecurrence(ctx context.Context, cmd CreateRecurrenceCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_attempt.recurrence")
	defer span.End()

	kind := interfaces.CategoryKindExpense
	if cmd.Direction == registerDirectionIncome {
		kind = interfaces.CategoryKindIncome
	}

	awaiting, candidates, catVersion, classifyErr := uc.classifyForPending(ctx, cmd.Description, kind, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.recurrence: classify: %w", classifyErr)
	}

	state := workflows.PendingEntryState{
		Status:               workflows.PendingStatusActive,
		Awaiting:             awaiting,
		OperationKind:        workflows.PendingOpCreateRecurrence,
		UserID:               cmd.UserID,
		ResourceID:           cmd.UserID,
		ThreadID:             cmd.ThreadID,
		MessageID:            cmd.WAMID,
		ItemSeq:              cmd.ItemSeq,
		AmountCents:          cmd.AmountCents,
		Description:          cmd.Description,
		PaymentMethod:        cmd.PaymentMethod,
		CardID:               cmd.CardID,
		OccurredAt:           resolveEntryDate(cmd.StartedAt),
		Kind:                 kind,
		Candidates:           candidates,
		CategoryVersion:      catVersion,
		Frequency:            cmd.Frequency,
		RecurrenceDayOfMonth: cmd.DayOfMonth,
	}

	key := workflows.PendingEntryKey(cmd.UserID.String(), cmd.ThreadID)
	startResult, err := uc.engine.Start(ctx, uc.def, key, state)
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.recurrence: start workflow: %w", err)
	}

	return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: pendingPrompt(startResult)}, nil
}

func (uc *RegisterAttempt) EditEntry(ctx context.Context, cmd EditEntryCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_attempt.edit")
	defer span.End()

	current, getErr := uc.ledger.GetTransaction(ctx, cmd.TargetTransactionID.String())
	if getErr != nil {
		span.RecordError(getErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.edit: obter lançamento: %w", getErr)
	}

	kind := interfaces.CategoryKindExpense
	if current.Direction == registerDirectionIncome {
		kind = interfaces.CategoryKindIncome
	}

	rootID, subID, idErr := currentCategoryIDs(current)
	if idErr != nil {
		span.RecordError(idErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.edit: %w", idErr)
	}

	searchRes, searchErr := uc.categories.SearchDictionary(ctx, current.Description, kind.String())
	if searchErr != nil {
		span.RecordError(searchErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.edit: versão editorial: %w", searchErr)
	}

	awaiting, candidates, catVersion, classifyErr := uc.classifyForPending(ctx, current.Description, kind, rootID, subID, searchRes.Version)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.edit: classify: %w", classifyErr)
	}

	state := workflows.PendingEntryState{
		Status:              workflows.PendingStatusActive,
		Awaiting:            awaiting,
		OperationKind:       workflows.PendingOpEditEntry,
		UserID:              cmd.UserID,
		ResourceID:          cmd.UserID,
		ThreadID:            cmd.ThreadID,
		MessageID:           cmd.WAMID,
		ItemSeq:             cmd.ItemSeq,
		AmountCents:         firstPositive(cmd.AmountCents, current.AmountCents),
		Description:         firstNonEmpty(cmd.Description, current.Description),
		PaymentMethod:       current.PaymentMethod,
		OccurredAt:          firstNonEmpty(cmd.OccurredAt, current.OccurredAt.Format("2006-01-02")),
		Kind:                kind,
		Candidates:          candidates,
		CategoryVersion:     catVersion,
		TargetTransactionID: &cmd.TargetTransactionID,
		TargetVersion:       current.Version,
	}

	key := workflows.PendingEntryKey(cmd.UserID.String(), cmd.ThreadID)
	startResult, startErr := uc.engine.Start(ctx, uc.def, key, state)
	if startErr != nil {
		span.RecordError(startErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_attempt.edit: start workflow: %w", startErr)
	}

	return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: pendingPrompt(startResult)}, nil
}

func pendingPrompt(result wf.RunResult[workflows.PendingEntryState]) string {
	if result.Suspend != nil && result.Suspend.Prompt != "" {
		return result.Suspend.Prompt
	}
	return result.State.ResponseText
}

func currentCategoryIDs(current interfaces.Entry) (uuid.UUID, uuid.UUID, error) {
	rootID, rootErr := uuid.Parse(current.CategoryID)
	if rootErr != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("categoryId inválido: %w", rootErr)
	}
	var subID uuid.UUID
	if current.SubcategoryID != nil {
		parsed, subErr := uuid.Parse(*current.SubcategoryID)
		if subErr != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("subcategoryId inválido: %w", subErr)
		}
		subID = parsed
	}
	return rootID, subID, nil
}

func firstPositive(override, fallback int64) int64 {
	if override > 0 {
		return override
	}
	return fallback
}

func firstNonEmpty(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func (uc *RegisterAttempt) classifyForPending(
	ctx context.Context,
	description string,
	kind interfaces.CategoryKind,
	rootID, subcategoryID uuid.UUID,
	version int64,
) (workflows.AwaitingSlot, []workflows.PendingCategoryCandidate, int64, error) {
	if subcategoryID != uuid.Nil {
		return uc.resolveExplicitForPending(ctx, kind, rootID, subcategoryID, version)
	}
	return uc.searchForPending(ctx, description, kind)
}

func (uc *RegisterAttempt) resolveExplicitForPending(
	ctx context.Context,
	kind interfaces.CategoryKind,
	rootID, subcategoryID uuid.UUID,
	version int64,
) (workflows.AwaitingSlot, []workflows.PendingCategoryCandidate, int64, error) {
	decision, err := uc.categories.ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
		RootCategoryID:  rootID,
		SubcategoryID:   subcategoryID,
		Kind:            kind,
		ExpectedVersion: version,
	})
	if err != nil {
		return workflows.AwaitingSlotCategory, nil, 0, nil
	}
	if decision.SubcategoryID == (uuid.UUID{}) || decision.SubcategoryID == decision.RootCategoryID {
		return workflows.AwaitingSlotCategory, nil, 0, nil
	}
	candidate := workflows.PendingCategoryCandidate{
		RootCategoryID:  decision.RootCategoryID,
		RootSlug:        decision.RootSlug,
		SubcategoryID:   decision.SubcategoryID,
		SubcategorySlug: decision.SubcategorySlug,
		Path:            decision.Path,
		Score:           1.0,
		Confidence:      "high",
		MatchQuality:    "exact",
		MatchReason:     "explicit",
	}
	return workflows.AwaitingSlotConfirmation, []workflows.PendingCategoryCandidate{candidate}, decision.EditorialVersion, nil
}

func (uc *RegisterAttempt) searchForPending(
	ctx context.Context,
	description string,
	kind interfaces.CategoryKind,
) (workflows.AwaitingSlot, []workflows.PendingCategoryCandidate, int64, error) {
	result, err := uc.categories.SearchDictionary(ctx, description, kind.String())
	if err != nil {
		return workflows.AwaitingSlotCategory, nil, 0, fmt.Errorf("search dictionary: %w", err)
	}
	candidates, enrichErr := workflows.EnrichCandidatesFromSearch(ctx, uc.categories, result, kind, result.Version)
	if enrichErr != nil {
		return workflows.AwaitingSlotCategory, nil, 0, fmt.Errorf("enrich candidates: %w", enrichErr)
	}
	if len(candidates) == 1 {
		return workflows.AwaitingSlotConfirmation, candidates, result.Version, nil
	}
	return workflows.AwaitingSlotCategory, candidates, result.Version, nil
}
