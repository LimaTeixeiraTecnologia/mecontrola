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
	registerMinInstallments     = 1
	registerMaxInstallments     = 24
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
	UserID        uuid.UUID
	WAMID         string
	ItemSeq       int
	AmountCents   int64
	Description   string
	PaymentMethod string
	OccurredAt    string
}

type RegisterIncomeCommand struct {
	UserID      uuid.UUID
	WAMID       string
	ItemSeq     int
	AmountCents int64
	Description string
	OccurredAt  string
}

type RegisterCardPurchaseCommand struct {
	UserID            uuid.UUID
	WAMID             string
	ItemSeq           int
	CardID            uuid.UUID
	TotalAmountCents  int64
	InstallmentsTotal int
	Description       string
	PurchasedAt       string
}

type RegisterEntry struct {
	categories interfaces.CategoriesReader
	ledger     interfaces.TransactionsLedger
	writer     registerWriter
	o11y       observability.Observability
}

func NewRegisterEntry(
	categories interfaces.CategoriesReader,
	ledger interfaces.TransactionsLedger,
	writer registerWriter,
	o11y observability.Observability,
) *RegisterEntry {
	return &RegisterEntry{categories: categories, ledger: ledger, writer: writer, o11y: o11y}
}

func (uc *RegisterEntry) RegisterExpense(ctx context.Context, cmd RegisterExpenseCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_expense")
	defer span.End()

	category, outcome, err := uc.classify(ctx, cmd.Description, registerCategoryKindExpense)
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_expense: %w", err)
	}
	if outcome == agent.ToolOutcomeClarify {
		return RegisterResult{Outcome: agent.ToolOutcomeClarify}, nil
	}

	occurredAt := resolveEntryDate(cmd.OccurredAt)
	subcategoryID := category.CategoryID
	result, err := uc.writer.Execute(ctx, cmd.UserID, cmd.WAMID, cmd.ItemSeq, "create_expense", "transaction", func(innerCtx context.Context) (uuid.UUID, bool, error) {
		ref, writeErr := uc.ledger.CreateTransaction(innerCtx, interfaces.RawTransaction{
			Direction:       registerDirectionOutcome,
			PaymentMethod:   cmd.PaymentMethod,
			AmountCents:     cmd.AmountCents,
			Description:     cmd.Description,
			OccurredAt:      occurredAt,
			CategoryID:      category.RootCategoryID,
			SubcategoryID:   &subcategoryID,
			OriginWamid:     cmd.WAMID,
			OriginItemSeq:   cmd.ItemSeq,
			OriginOperation: "create_expense",
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

	category, outcome, err := uc.classify(ctx, cmd.Description, registerCategoryKindIncome)
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_income: %w", err)
	}
	if outcome == agent.ToolOutcomeClarify {
		return RegisterResult{Outcome: agent.ToolOutcomeClarify}, nil
	}

	occurredAt := resolveEntryDate(cmd.OccurredAt)
	subcategoryID := category.CategoryID
	result, err := uc.writer.Execute(ctx, cmd.UserID, cmd.WAMID, cmd.ItemSeq, "create_income", "transaction", func(innerCtx context.Context) (uuid.UUID, bool, error) {
		ref, writeErr := uc.ledger.CreateTransaction(innerCtx, interfaces.RawTransaction{
			Direction:       registerDirectionIncome,
			PaymentMethod:   registerIncomePaymentMethod,
			AmountCents:     cmd.AmountCents,
			Description:     cmd.Description,
			OccurredAt:      occurredAt,
			CategoryID:      category.RootCategoryID,
			SubcategoryID:   &subcategoryID,
			OriginWamid:     cmd.WAMID,
			OriginItemSeq:   cmd.ItemSeq,
			OriginOperation: "create_income",
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

func (uc *RegisterEntry) RegisterCardPurchase(ctx context.Context, cmd RegisterCardPurchaseCommand) (RegisterResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.register_card_purchase")
	defer span.End()

	if cmd.InstallmentsTotal < registerMinInstallments || cmd.InstallmentsTotal > registerMaxInstallments {
		err := fmt.Errorf("agents.usecase.register_card_purchase: installments_total: fora do intervalo %d..%d, recebido %d", registerMinInstallments, registerMaxInstallments, cmd.InstallmentsTotal)
		span.RecordError(err)
		return RegisterResult{}, err
	}

	category, outcome, err := uc.classify(ctx, cmd.Description, registerCategoryKindExpense)
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_card_purchase: %w", err)
	}
	if outcome == agent.ToolOutcomeClarify {
		return RegisterResult{Outcome: agent.ToolOutcomeClarify}, nil
	}

	purchasedAt := resolveEntryDate(cmd.PurchasedAt)
	subcategoryID := category.CategoryID
	result, err := uc.writer.Execute(ctx, cmd.UserID, cmd.WAMID, cmd.ItemSeq, "create_card_purchase", "card_purchase", func(innerCtx context.Context) (uuid.UUID, bool, error) {
		ref, writeErr := uc.ledger.CreateCardPurchase(innerCtx, interfaces.RawCardPurchase{
			CardID:            cmd.CardID,
			TotalAmountCents:  cmd.TotalAmountCents,
			InstallmentsTotal: cmd.InstallmentsTotal,
			Description:       cmd.Description,
			CategoryID:        category.RootCategoryID,
			SubcategoryID:     &subcategoryID,
			PurchasedAt:       purchasedAt,
			OriginWamid:       cmd.WAMID,
			OriginItemSeq:     cmd.ItemSeq,
			OriginOperation:   "create_card_purchase",
		})
		if writeErr != nil {
			return uuid.Nil, false, writeErr
		}
		return ref.ID, ref.Reconciled, nil
	})
	if err != nil {
		span.RecordError(err)
		return RegisterResult{}, fmt.Errorf("agents.usecase.register_card_purchase: %w", err)
	}
	return RegisterResult{ResourceID: result.ResourceID, Kind: "card_purchase", Outcome: result.Outcome}, nil
}

func (uc *RegisterEntry) classify(ctx context.Context, description, kind string) (interfaces.CategoryCandidate, agent.ToolOutcome, error) {
	candidates, err := uc.categories.SearchDictionary(ctx, description, kind)
	if err != nil {
		return interfaces.CategoryCandidate{}, agent.ToolOutcomeUsecaseError, fmt.Errorf("classify %q: %w", kind, err)
	}
	if len(candidates) == 0 {
		return interfaces.CategoryCandidate{}, agent.ToolOutcomeClarify, nil
	}
	top := candidates[0]
	if len(candidates) > 1 || top.IsAmbiguous {
		return interfaces.CategoryCandidate{}, agent.ToolOutcomeClarify, nil
	}
	return top, agent.ToolOutcomeRouted, nil
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
