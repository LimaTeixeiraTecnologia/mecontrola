package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const (
	registerCategoryKindExpense = "expense"
	registerCategoryKindIncome  = "income"
	registerDirectionOutcome    = "outcome"
	registerDirectionIncome     = "income"
	registerIncomePaymentMethod = "pix"
	registerCategorySource      = "auto_matched"

	registerUserSelectedSource      = "user_selected_candidate"
	registerUserSelectedScore       = 1.0
	registerUserSelectedConfidence  = "high"
	registerUserSelectedQuality     = "exact"
	registerUserSelectedSignalType  = "canonical_name"
	registerUserSelectedMatchReason = "seleção explícita do usuário via classify_category"
)

type registerWriter interface {
	Execute(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write WriteFn) (IdempotentWriteResult, error)
}

type RegisterResult struct {
	ResourceID uuid.UUID
	Kind       string
	Outcome    agent.ToolOutcome
}

type RegisterExpenseCommand struct {
	UserID          uuid.UUID
	WAMID           string
	ItemSeq         int
	AmountCents     int64
	Description     string
	PaymentMethod   string
	CardID          *uuid.UUID
	Installments    int
	OccurredAt      string
	CategoryID      uuid.UUID
	SubcategoryID   uuid.UUID
	CategoryVersion int64
}

type RegisterIncomeCommand struct {
	UserID          uuid.UUID
	WAMID           string
	ItemSeq         int
	AmountCents     int64
	Description     string
	OccurredAt      string
	CategoryID      uuid.UUID
	SubcategoryID   uuid.UUID
	CategoryVersion int64
}

type classifyResult struct {
	Candidate interfaces.CategoryCandidate
	Version   int64
	Source    string
}

type RegisterEntry struct {
	categories  interfaces.CategoriesReader
	ledger      interfaces.TransactionsLedger
	writer      registerWriter
	o11y        observability.Observability
	clarifTotal observability.Counter
}

func NewRegisterEntry(
	categories interfaces.CategoriesReader,
	ledger interfaces.TransactionsLedger,
	writer registerWriter,
	o11y observability.Observability,
) *RegisterEntry {
	return &RegisterEntry{
		categories: categories,
		ledger:     ledger,
		writer:     writer,
		o11y:       o11y,
		clarifTotal: o11y.Metrics().Counter(
			"category_clarification_requested_total",
			"Total de clarificacoes de categoria solicitadas ao usuario",
			"1",
		),
	}
}

func (uc *RegisterEntry) RegisterExpense(ctx context.Context, cmd RegisterExpenseCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_expense")
	defer span.End()

	cr, outcome, err := uc.resolveCategory(ctx, cmd.Description, registerCategoryKindExpense, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_expense: %w", err)
	}
	if outcome == agent.ToolOutcomeClarify {
		return RegisterResult{Outcome: agent.ToolOutcomeClarify}, nil
	}

	occurredAt := resolveEntryDate(cmd.OccurredAt)
	subcategoryID := cr.Candidate.CategoryID
	result, err := uc.writer.Execute(ctx, cmd.UserID, cmd.WAMID, cmd.ItemSeq, "create_expense", "transaction", func(innerCtx context.Context) (uuid.UUID, bool, error) {
		ref, writeErr := uc.ledger.CreateTransaction(innerCtx, interfaces.RawTransaction{
			Direction:           registerDirectionOutcome,
			PaymentMethod:       cmd.PaymentMethod,
			AmountCents:         cmd.AmountCents,
			Description:         cmd.Description,
			OccurredAt:          occurredAt,
			CategoryID:          cr.Candidate.RootCategoryID,
			SubcategoryID:       &subcategoryID,
			CardID:              cmd.CardID,
			Installments:        cmd.Installments,
			OriginWamid:         cmd.WAMID,
			OriginItemSeq:       cmd.ItemSeq,
			OriginOperation:     "create_expense",
			CategorySource:      cr.Source,
			CategoryOutcome:     "matched",
			CategoryScore:       cr.Candidate.Score,
			CategoryConfidence:  cr.Candidate.Confidence,
			CategoryQuality:     cr.Candidate.MatchQuality,
			CategorySignalType:  cr.Candidate.SignalType,
			CategoryMatchedTerm: cr.Candidate.MatchedTerm,
			CategoryMatchReason: cr.Candidate.MatchReason,
			CategoryVersion:     cr.Version,
		})
		if writeErr != nil {
			return uuid.Nil, false, writeErr
		}
		return ref.ID, ref.Reconciled, nil
	})
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_expense: %w", err)
	}
	return RegisterResult{ResourceID: result.ResourceID, Kind: "transaction", Outcome: result.Outcome}, nil
}

func (uc *RegisterEntry) RegisterIncome(ctx context.Context, cmd RegisterIncomeCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_income")
	defer span.End()

	cr, outcome, err := uc.resolveCategory(ctx, cmd.Description, registerCategoryKindIncome, cmd.CategoryID, cmd.SubcategoryID, cmd.CategoryVersion)
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_income: %w", err)
	}
	if outcome == agent.ToolOutcomeClarify {
		return RegisterResult{Outcome: agent.ToolOutcomeClarify}, nil
	}

	occurredAt := resolveEntryDate(cmd.OccurredAt)
	subcategoryID := cr.Candidate.CategoryID
	result, err := uc.writer.Execute(ctx, cmd.UserID, cmd.WAMID, cmd.ItemSeq, "create_income", "transaction", func(innerCtx context.Context) (uuid.UUID, bool, error) {
		ref, writeErr := uc.ledger.CreateTransaction(innerCtx, interfaces.RawTransaction{
			Direction:           registerDirectionIncome,
			PaymentMethod:       registerIncomePaymentMethod,
			AmountCents:         cmd.AmountCents,
			Description:         cmd.Description,
			OccurredAt:          occurredAt,
			CategoryID:          cr.Candidate.RootCategoryID,
			SubcategoryID:       &subcategoryID,
			OriginWamid:         cmd.WAMID,
			OriginItemSeq:       cmd.ItemSeq,
			OriginOperation:     "create_income",
			CategorySource:      cr.Source,
			CategoryOutcome:     "matched",
			CategoryScore:       cr.Candidate.Score,
			CategoryConfidence:  cr.Candidate.Confidence,
			CategoryQuality:     cr.Candidate.MatchQuality,
			CategorySignalType:  cr.Candidate.SignalType,
			CategoryMatchedTerm: cr.Candidate.MatchedTerm,
			CategoryMatchReason: cr.Candidate.MatchReason,
			CategoryVersion:     cr.Version,
		})
		if writeErr != nil {
			return uuid.Nil, false, writeErr
		}
		return ref.ID, ref.Reconciled, nil
	})
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_income: %w", err)
	}
	return RegisterResult{ResourceID: result.ResourceID, Kind: "transaction", Outcome: result.Outcome}, nil
}

func (uc *RegisterEntry) classify(ctx context.Context, description, kind string) (classifyResult, agent.ToolOutcome, error) {
	result, err := uc.categories.SearchDictionary(ctx, description, kind)
	if err != nil {
		return classifyResult{}, agent.ToolOutcomeUsecaseError, fmt.Errorf("classify %q: %w", kind, err)
	}
	if result.Version <= 0 {
		return classifyResult{}, agent.ToolOutcomeUsecaseError, fmt.Errorf("classify %q: versão editorial ausente", kind)
	}
	if result.Outcome != interfaces.ClassifyOutcomeMatched {
		uc.clarifTotal.Add(ctx, 1,
			observability.String("reason", result.Outcome.String()),
			observability.String("kind", kind),
			observability.String("surface", "register_entry"),
		)
		return classifyResult{}, agent.ToolOutcomeClarify, nil
	}
	if len(result.Candidates) != 1 {
		uc.clarifTotal.Add(ctx, 1,
			observability.String("reason", "multi_candidate"),
			observability.String("kind", kind),
			observability.String("surface", "register_entry"),
		)
		return classifyResult{}, agent.ToolOutcomeClarify, nil
	}
	candidate := result.Candidates[0]
	if candidate.IsAmbiguous {
		uc.clarifTotal.Add(ctx, 1,
			observability.String("reason", "ambiguous"),
			observability.String("kind", kind),
			observability.String("surface", "register_entry"),
		)
		return classifyResult{}, agent.ToolOutcomeClarify, nil
	}
	if candidate.RootCategoryID == candidate.CategoryID {
		uc.clarifTotal.Add(ctx, 1,
			observability.String("reason", "no_leaf"),
			observability.String("kind", kind),
			observability.String("surface", "register_entry"),
		)
		return classifyResult{}, agent.ToolOutcomeClarify, nil
	}
	if candidate.Confidence == "" || candidate.MatchQuality == "" {
		uc.clarifTotal.Add(ctx, 1,
			observability.String("reason", "low_quality"),
			observability.String("kind", kind),
			observability.String("surface", "register_entry"),
		)
		return classifyResult{}, agent.ToolOutcomeClarify, nil
	}
	return classifyResult{Candidate: candidate, Version: result.Version, Source: registerCategorySource}, agent.ToolOutcomeRouted, nil
}

func (uc *RegisterEntry) resolveCategory(ctx context.Context, description, kind string, rootID, subcategoryID uuid.UUID, version int64) (classifyResult, agent.ToolOutcome, error) {
	if subcategoryID == uuid.Nil {
		return uc.classify(ctx, description, kind)
	}
	return uc.resolveExplicit(ctx, kind, rootID, subcategoryID, version)
}

func (uc *RegisterEntry) resolveExplicit(ctx context.Context, kind string, rootID, subcategoryID uuid.UUID, version int64) (classifyResult, agent.ToolOutcome, error) {
	categoryKind, err := interfaces.ParseCategoryKind(kind)
	if err != nil {
		return classifyResult{}, agent.ToolOutcomeUsecaseError, fmt.Errorf("resolve explicit %q: %w", kind, err)
	}
	decision, err := uc.categories.ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
		RootCategoryID:  rootID,
		SubcategoryID:   subcategoryID,
		Kind:            categoryKind,
		ExpectedVersion: version,
	})
	if err != nil {
		uc.clarifTotal.Add(ctx, 1,
			observability.String("reason", "explicit_resolve_failed"),
			observability.String("kind", kind),
			observability.String("surface", "register_entry"),
		)
		return classifyResult{}, agent.ToolOutcomeClarify, nil
	}
	return classifyResult{
		Candidate: interfaces.CategoryCandidate{
			CategoryID:     decision.SubcategoryID,
			RootCategoryID: decision.RootCategoryID,
			Path:           decision.Path,
			Score:          registerUserSelectedScore,
			Confidence:     registerUserSelectedConfidence,
			MatchQuality:   registerUserSelectedQuality,
			SignalType:     registerUserSelectedSignalType,
			MatchedTerm:    decision.Path,
			MatchReason:    registerUserSelectedMatchReason,
		},
		Version: decision.EditorialVersion,
		Source:  registerUserSelectedSource,
	}, agent.ToolOutcomeRouted, nil
}

func resolveEntryDate(raw string) string {
	if raw != "" {
		return raw
	}
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}
