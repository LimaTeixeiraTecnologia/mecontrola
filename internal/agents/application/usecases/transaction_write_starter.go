package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	catinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type TransactionWriteStarter struct {
	categories interfaces.CategoriesReader
	ledger     interfaces.TransactionsLedger
	engine     wf.Engine[workflows.TransactionWriteState]
	def        wf.Definition[workflows.TransactionWriteState]
	o11y       observability.Observability
}

func NewTransactionWriteStarter(
	categories interfaces.CategoriesReader,
	ledger interfaces.TransactionsLedger,
	engine wf.Engine[workflows.TransactionWriteState],
	def wf.Definition[workflows.TransactionWriteState],
	o11y observability.Observability,
) *TransactionWriteStarter {
	return &TransactionWriteStarter{
		categories: categories,
		ledger:     ledger,
		engine:     engine,
		def:        def,
		o11y:       o11y,
	}
}

func (uc *TransactionWriteStarter) RegisterExpense(ctx context.Context, cmd RegisterExpenseCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.transaction_write_starter.expense")
	defer span.End()

	cmd.Description = workflows.NormalizeEntryDescription(cmd.Description)

	candidates, catVersion, classifyErr := uc.classify(ctx, cmd.Description, interfaces.CategoryKindExpense, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.transaction_write_starter.expense: classify: %w", classifyErr)
	}

	installments := cmd.Installments
	if installments <= 0 {
		installments = 1
	}

	state := workflows.TransactionWriteState{
		Status:          workflows.TransactionWriteStatusActive,
		OperationKind:   workflows.TransactionOpRegisterExpense,
		UserID:          cmd.UserID,
		ResourceID:      cmd.UserID,
		ThreadID:        cmd.ThreadID,
		MessageID:       cmd.WAMID,
		ItemSeq:         cmd.ItemSeq,
		AmountCents:     cmd.AmountCents,
		Description:     cmd.Description,
		PaymentMethod:   cmd.PaymentMethod,
		CardID:          cmd.CardID,
		Installments:    installments,
		OccurredAt:      resolveEntryDate(cmd.OccurredAt),
		Kind:            interfaces.CategoryKindExpense,
		Candidates:      candidates,
		CategoryVersion: catVersion,
	}

	return uc.start(ctx, span, cmd.UserID, cmd.ThreadID, state)
}

func (uc *TransactionWriteStarter) RegisterIncome(ctx context.Context, cmd RegisterIncomeCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.transaction_write_starter.income")
	defer span.End()

	cmd.Description = workflows.NormalizeEntryDescription(cmd.Description)

	candidates, catVersion, classifyErr := uc.classify(ctx, cmd.Description, interfaces.CategoryKindIncome, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.transaction_write_starter.income: classify: %w", classifyErr)
	}

	state := workflows.TransactionWriteState{
		Status:          workflows.TransactionWriteStatusActive,
		OperationKind:   workflows.TransactionOpRegisterIncome,
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

	return uc.start(ctx, span, cmd.UserID, cmd.ThreadID, state)
}

func (uc *TransactionWriteStarter) CreateRecurrence(ctx context.Context, cmd CreateRecurrenceCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.transaction_write_starter.recurrence")
	defer span.End()

	cmd.Description = workflows.NormalizeEntryDescription(cmd.Description)

	kind := interfaces.CategoryKindExpense
	if cmd.Direction == registerDirectionIncome {
		kind = interfaces.CategoryKindIncome
	}

	candidates, catVersion, classifyErr := uc.classify(ctx, cmd.Description, kind, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if classifyErr != nil {
		span.RecordError(classifyErr)
		return RegisterResult{}, fmt.Errorf("agents.usecase.transaction_write_starter.recurrence: classify: %w", classifyErr)
	}

	state := workflows.TransactionWriteState{
		Status:               workflows.TransactionWriteStatusActive,
		OperationKind:        workflows.TransactionOpCreateRecurrence,
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

	return uc.start(ctx, span, cmd.UserID, cmd.ThreadID, state)
}

func (uc *TransactionWriteStarter) EditEntry(ctx context.Context, cmd EditEntryCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.transaction_write_starter.edit")
	defer span.End()

	cmd.Description = workflows.NormalizeEntryDescription(cmd.Description)

	state := workflows.TransactionWriteState{
		Status:        workflows.TransactionWriteStatusActive,
		OperationKind: workflows.TransactionOpEditEntry,
		UserID:        cmd.UserID,
		ResourceID:    cmd.UserID,
		ThreadID:      cmd.ThreadID,
		MessageID:     cmd.WAMID,
		ItemSeq:       cmd.ItemSeq,
	}

	if cmd.TargetTransactionID != uuid.Nil {
		if err := uc.populateEditTarget(ctx, &state, cmd); err != nil {
			span.RecordError(err)
			return RegisterResult{}, fmt.Errorf("agents.usecase.transaction_write_starter.edit: %w", err)
		}
	} else {
		state.EditSearchAmountCents = cmd.SearchAmountCents
		state.EditSearchTerm = cmd.SearchTerm
	}

	return uc.start(ctx, span, cmd.UserID, cmd.ThreadID, state)
}

func (uc *TransactionWriteStarter) populateEditTarget(ctx context.Context, state *workflows.TransactionWriteState, cmd EditEntryCommand) error {
	current, getErr := uc.ledger.GetTransaction(ctx, cmd.TargetTransactionID.String())
	if getErr != nil {
		return fmt.Errorf("obter lançamento: %w", getErr)
	}

	kind := interfaces.CategoryKindExpense
	if current.Direction == registerDirectionIncome {
		kind = interfaces.CategoryKindIncome
	}
	state.Kind = kind

	txID := cmd.TargetTransactionID
	state.TargetTransactionID = &txID
	state.TargetVersion = current.Version

	rootID, subID, idErr := currentCategoryIDs(current)
	if idErr != nil {
		return idErr
	}
	state.TargetCategoryID = rootID
	if current.SubcategoryID != nil {
		sub := subID
		state.TargetSubcategoryID = &sub
	}
	state.TargetPaymentMethod = current.PaymentMethod
	state.TargetDescription = current.Description
	state.TargetOccurredAt = current.OccurredAt.Format("2006-01-02")

	state.AmountCents = cmd.AmountCents
	state.Description = cmd.Description
	state.OccurredAt = cmd.OccurredAt
	state.PaymentMethod = cmd.PaymentMethod

	if cmd.SubcategoryID != uuid.Nil {
		candidates, catVersion, classifyErr := uc.resolveExplicit(ctx, kind, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
		if classifyErr != nil {
			return fmt.Errorf("classify: %w", classifyErr)
		}
		state.Candidates = candidates
		state.CategoryVersion = catVersion
	}

	return nil
}

func (uc *TransactionWriteStarter) start(
	ctx context.Context,
	span observability.Span,
	userID uuid.UUID,
	threadID string,
	state workflows.TransactionWriteState,
) (RegisterResult, error) {
	key := workflows.TransactionWriteKey(userID.String(), threadID)
	startResult, err := uc.engine.Start(ctx, uc.def, key, state)
	if err != nil {
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: messages.ActiveWriteExists()}, nil
		}
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.transaction_write_starter: start workflow: %w", err)
	}
	return RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: transactionWritePrompt(startResult)}, nil
}

func transactionWritePrompt(result wf.RunResult[workflows.TransactionWriteState]) string {
	if result.Suspend != nil && result.Suspend.Prompt != "" {
		return result.Suspend.Prompt
	}
	return result.State.ResponseText
}

func (uc *TransactionWriteStarter) classify(
	ctx context.Context,
	description string,
	kind interfaces.CategoryKind,
	rootID, subcategoryID uuid.UUID,
	version int64,
) ([]workflows.PendingCategoryCandidate, int64, error) {
	if subcategoryID != uuid.Nil {
		return uc.resolveExplicit(ctx, kind, rootID, subcategoryID, version)
	}
	return uc.searchByDescription(ctx, description, kind)
}

func (uc *TransactionWriteStarter) resolveExplicit(
	ctx context.Context,
	kind interfaces.CategoryKind,
	rootID, subcategoryID uuid.UUID,
	version int64,
) ([]workflows.PendingCategoryCandidate, int64, error) {
	decision, err := uc.categories.ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
		RootCategoryID:  rootID,
		SubcategoryID:   subcategoryID,
		Kind:            kind,
		ExpectedVersion: version,
	})
	if err != nil {
		return nil, 0, nil
	}
	if decision.SubcategoryID == (uuid.UUID{}) || decision.SubcategoryID == decision.RootCategoryID {
		return nil, 0, nil
	}
	candidate := workflows.PendingCategoryCandidate{
		RootCategoryID:  decision.RootCategoryID,
		RootSlug:        decision.RootSlug,
		SubcategoryID:   decision.SubcategoryID,
		SubcategorySlug: decision.SubcategorySlug,
		Path:            decision.Path,
		Score:           1.0,
		Confidence:      "manual_confirmed",
		MatchQuality:    "manual_canonical",
		SignalType:      "manual_canonical",
		MatchedTerm:     decision.SubcategorySlug,
		MatchReason:     "manual canonical id validated",
	}
	return []workflows.PendingCategoryCandidate{candidate}, decision.EditorialVersion, nil
}

func (uc *TransactionWriteStarter) searchByDescription(
	ctx context.Context,
	description string,
	kind interfaces.CategoryKind,
) ([]workflows.PendingCategoryCandidate, int64, error) {
	result, err := uc.categories.SearchDictionary(ctx, description, kind.String())
	if err != nil {
		if errors.Is(err, catinput.ErrInvalidQuery) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("search dictionary: %w", err)
	}
	candidates, enrichErr := workflows.EnrichCandidatesFromSearch(ctx, uc.categories, result, kind, result.Version)
	if enrichErr != nil {
		return nil, 0, fmt.Errorf("enrich candidates: %w", enrichErr)
	}
	return candidates, result.Version, nil
}
