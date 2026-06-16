package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var (
	ErrLogExpenseInvalidIntent     = errors.New("agent: log expense: intent invalido")
	ErrLogExpenseNoCategoryHint    = errors.New("agent: log expense: sem hint de categoria")
	ErrLogExpenseCategoryAmbiguous = errors.New("agent: log expense: categoria ambigua")
	ErrLogExpenseCategoryNotFound  = errors.New("agent: log expense: categoria nao encontrada")
)

type CategoryResolver interface {
	Execute(ctx context.Context, in *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error)
}

type ExpenseUpserter interface {
	Execute(ctx context.Context, in ExpenseUpsertInput) (ExpenseUpsertResult, error)
}

type ExpenseUpsertInput struct {
	UserID                string
	Source                string
	ExternalTransactionID string
	SubcategoryID         string
	Competence            string
	AmountCents           int64
	OccurredAt            time.Time
}

type ExpenseUpsertResult struct {
	ID             string
	UserID         string
	SubcategoryID  string
	RootCategoryID string
	Competence     string
	AmountCents    int64
	OccurredAt     time.Time
	Version        int64
}

type LogExpenseFromAgent struct {
	resolver   CategoryResolver
	upserter   ExpenseUpserter
	source     string
	loc        *time.Location
	o11y       observability.Observability
	persisted  observability.Counter
	resolveBad observability.Counter
}

func NewLogExpenseFromAgent(
	resolver CategoryResolver,
	upserter ExpenseUpserter,
	loc *time.Location,
	o11y observability.Observability,
) *LogExpenseFromAgent {
	if loc == nil {
		loc = time.UTC
	}
	persisted := o11y.Metrics().Counter(
		"agent_log_expense_persisted_total",
		"Total de expenses persistidas a partir de intent LogExpense",
		"1",
	)
	resolveBad := o11y.Metrics().Counter(
		"agent_log_expense_resolve_failed_total",
		"Total de tentativas de log expense que falharam ao resolver categoria",
		"1",
	)
	return &LogExpenseFromAgent{
		resolver:   resolver,
		upserter:   upserter,
		source:     "agent",
		loc:        loc,
		o11y:       o11y,
		persisted:  persisted,
		resolveBad: resolveBad,
	}
}

type LogExpenseFromAgentInput struct {
	UserID string
	Intent intent.Intent
}

type LogExpenseFromAgentResult struct {
	Persisted      bool
	SubcategoryID  string
	RootCategoryID string
	AmountCents    int64
	Competence     string
	CategoryPath   string
	OccurredAt     time.Time
}

func (uc *LogExpenseFromAgent) Execute(ctx context.Context, in LogExpenseFromAgentInput) (LogExpenseFromAgentResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.log_expense_from_agent")
	defer span.End()

	if in.Intent.Kind() != intent.KindLogExpense {
		return LogExpenseFromAgentResult{}, ErrLogExpenseInvalidIntent
	}
	if strings.TrimSpace(in.UserID) == "" {
		return LogExpenseFromAgentResult{}, errors.New("agent: log expense: user id vazio")
	}
	if in.Intent.AmountCents() <= 0 {
		return LogExpenseFromAgentResult{}, errors.New("agent: log expense: amount invalido")
	}

	hint := strings.TrimSpace(in.Intent.CategoryHint())
	if hint == "" {
		hint = strings.TrimSpace(in.Intent.Merchant())
	}
	if hint == "" {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_hint"))
		return LogExpenseFromAgentResult{}, ErrLogExpenseNoCategoryHint
	}

	candidate, path, err := uc.resolve(ctx, hint)
	if err != nil {
		return LogExpenseFromAgentResult{}, err
	}

	now := time.Now()
	if uc.loc != nil {
		now = now.In(uc.loc)
	}
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))

	out, err := uc.upserter.Execute(ctx, ExpenseUpsertInput{
		UserID:                in.UserID,
		Source:                uc.source,
		ExternalTransactionID: uuid.NewString(),
		SubcategoryID:         candidate.CategoryID.String(),
		Competence:            competence,
		AmountCents:           in.Intent.AmountCents(),
		OccurredAt:            now.UTC(),
	})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "upsert_failed"))
		return LogExpenseFromAgentResult{}, fmt.Errorf("agent: log expense: upsert: %w", err)
	}

	uc.persisted.Add(ctx, 1, observability.String("category_id", out.SubcategoryID))
	return LogExpenseFromAgentResult{
		Persisted:      true,
		SubcategoryID:  out.SubcategoryID,
		RootCategoryID: out.RootCategoryID,
		AmountCents:    out.AmountCents,
		Competence:     out.Competence,
		CategoryPath:   path,
		OccurredAt:     out.OccurredAt,
	}, nil
}

func (uc *LogExpenseFromAgent) resolve(ctx context.Context, hint string) (categoriesoutput.CandidateOutput, string, error) {
	result, err := uc.resolver.Execute(ctx, &categoriesinput.SearchDictionaryInput{Query: hint, Kind: categoriesvo.KindExpense})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "resolver_failed"))
		return categoriesoutput.CandidateOutput{}, "", fmt.Errorf("agent: log expense: resolver: %w", err)
	}
	if result == nil || len(result.Candidates) == 0 {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_match"))
		return categoriesoutput.CandidateOutput{}, "", ErrLogExpenseCategoryNotFound
	}
	top := result.Candidates[0]
	if top.IsAmbiguous && len(result.Candidates) > 1 {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "ambiguous"))
		return categoriesoutput.CandidateOutput{}, "", ErrLogExpenseCategoryAmbiguous
	}
	return top, top.Path, nil
}
