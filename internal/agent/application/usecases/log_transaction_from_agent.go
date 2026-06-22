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
	ErrLogTransactionInvalidIntent     = errors.New("agent: log transaction: intent invalido")
	ErrLogTransactionNoCategoryHint    = errors.New("agent: log transaction: sem hint de categoria")
	ErrLogTransactionCategoryAmbiguous = errors.New("agent: log transaction: categoria ambigua")
	ErrLogTransactionCategoryNotFound  = errors.New("agent: log transaction: categoria nao encontrada")
)

type CategoryAmbiguousError struct {
	Hint       string
	Candidates []string
}

func (e *CategoryAmbiguousError) Error() string {
	return fmt.Sprintf("%s: hint=%q candidatos=%s", ErrLogTransactionCategoryAmbiguous.Error(), e.Hint, strings.Join(e.Candidates, ", "))
}

func (e *CategoryAmbiguousError) Unwrap() error {
	return ErrLogTransactionCategoryAmbiguous
}

func newCategoryAmbiguousError(hint string, candidates []categoriesoutput.CandidateOutput) *CategoryAmbiguousError {
	limit := len(candidates)
	if limit > 3 {
		limit = 3
	}
	paths := make([]string, 0, limit)
	for _, candidate := range candidates[:limit] {
		path := strings.TrimSpace(candidate.Path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return &CategoryAmbiguousError{Hint: strings.TrimSpace(hint), Candidates: paths}
}

const (
	directionOutcome  = "outcome"
	directionIncome   = "income"
	defaultIncomeHint = "salário"
)

type CategoryResolver interface {
	Execute(ctx context.Context, in *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error)
}

type TransactionCreator interface {
	Execute(ctx context.Context, in CreateTransactionCommand) (CreateTransactionResult, error)
}

type CreateTransactionCommand struct {
	UserID         string
	Direction      string
	PaymentMethod  string
	Description    string
	RootCategoryID string
	SubcategoryID  string
	AmountCents    int64
	OccurredAt     time.Time
}

type CreateTransactionResult struct {
	AmountCents int64
	Direction   string
}

type LogTransactionFromAgent struct {
	resolver   CategoryResolver
	creator    TransactionCreator
	o11y       observability.Observability
	persisted  observability.Counter
	resolveBad observability.Counter
}

func NewLogTransactionFromAgent(
	resolver CategoryResolver,
	creator TransactionCreator,
	o11y observability.Observability,
) *LogTransactionFromAgent {
	persisted := o11y.Metrics().Counter(
		"agent_log_transaction_persisted_total",
		"Total de transações persistidas a partir de intent do agente por direction",
		"1",
	)
	resolveBad := o11y.Metrics().Counter(
		"agent_log_transaction_resolve_failed_total",
		"Total de tentativas de log de transação que falharam ao resolver categoria ou persistir",
		"1",
	)
	return &LogTransactionFromAgent{
		resolver:   resolver,
		creator:    creator,
		o11y:       o11y,
		persisted:  persisted,
		resolveBad: resolveBad,
	}
}

type LogTransactionFromAgentInput struct {
	UserID string
	Intent intent.Intent
}

type LogTransactionFromAgentResult struct {
	Persisted    bool
	AmountCents  int64
	Direction    string
	CategoryPath string
	OccurredAt   time.Time
}

func (uc *LogTransactionFromAgent) Execute(ctx context.Context, in LogTransactionFromAgentInput) (LogTransactionFromAgentResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.log_transaction_from_agent")
	defer span.End()

	direction, categoryKind, err := directionForKind(in.Intent.Kind())
	if err != nil {
		return LogTransactionFromAgentResult{}, err
	}
	if strings.TrimSpace(in.UserID) == "" {
		return LogTransactionFromAgentResult{}, errors.New("agent: log transaction: user id vazio")
	}
	if in.Intent.AmountCents() <= 0 {
		return LogTransactionFromAgentResult{}, errors.New("agent: log transaction: amount invalido")
	}

	hint := strings.TrimSpace(in.Intent.CategoryHint())
	if hint == "" {
		hint = strings.TrimSpace(in.Intent.Merchant())
	}
	if hint == "" {
		if direction != directionIncome {
			uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_hint"))
			return LogTransactionFromAgentResult{}, ErrLogTransactionNoCategoryHint
		}
		hint = defaultIncomeHint
	}

	candidate, path, err := uc.resolve(ctx, hint, categoryKind)
	if err != nil {
		return LogTransactionFromAgentResult{}, err
	}

	now := time.Now().UTC()
	description := strings.TrimSpace(in.Intent.Merchant())
	if description == "" {
		description = path
	}

	result, err := uc.creator.Execute(ctx, CreateTransactionCommand{
		UserID:         in.UserID,
		Direction:      direction,
		PaymentMethod:  mapPaymentMethod(in.Intent.PaymentMethod(), direction),
		Description:    description,
		RootCategoryID: candidate.RootCategoryID.String(),
		SubcategoryID:  subcategoryID(candidate),
		AmountCents:    in.Intent.AmountCents(),
		OccurredAt:     now,
	})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "create_failed"))
		return LogTransactionFromAgentResult{}, fmt.Errorf("agent: log transaction: create: %w", err)
	}

	uc.persisted.Add(ctx, 1, observability.String("direction", direction))
	return LogTransactionFromAgentResult{
		Persisted:    true,
		AmountCents:  result.AmountCents,
		Direction:    result.Direction,
		CategoryPath: path,
		OccurredAt:   now,
	}, nil
}

func (uc *LogTransactionFromAgent) resolve(ctx context.Context, hint string, kind categoriesvo.Kind) (categoriesoutput.CandidateOutput, string, error) {
	result, err := uc.resolver.Execute(ctx, &categoriesinput.SearchDictionaryInput{Query: hint, Kind: kind})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "resolver_failed"))
		return categoriesoutput.CandidateOutput{}, "", fmt.Errorf("agent: log transaction: resolver: %w", err)
	}
	if result == nil || len(result.Candidates) == 0 {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_match"))
		return categoriesoutput.CandidateOutput{}, "", ErrLogTransactionCategoryNotFound
	}
	top := result.Candidates[0]
	if top.IsAmbiguous && len(result.Candidates) > 1 {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "ambiguous"))
		return categoriesoutput.CandidateOutput{}, "", newCategoryAmbiguousError(hint, result.Candidates)
	}
	return top, top.Path, nil
}

func directionForKind(kind intent.Kind) (string, categoriesvo.Kind, error) {
	switch kind {
	case intent.KindLogExpense:
		return directionOutcome, categoriesvo.KindExpense, nil
	case intent.KindLogIncome:
		return directionIncome, categoriesvo.KindIncome, nil
	default:
		return "", 0, ErrLogTransactionInvalidIntent
	}
}

func mapPaymentMethod(intentMethod, direction string) string {
	switch strings.ToLower(strings.TrimSpace(intentMethod)) {
	case "pix":
		return "pix"
	case "credit":
		return "credit_card"
	case "debit":
		return "debit_card"
	case "cash":
		return "cash"
	case "transfer":
		return "ted"
	case "boleto":
		return "boleto"
	}
	if direction == directionIncome {
		return "ted"
	}
	return "pix"
}

func subcategoryID(candidate categoriesoutput.CandidateOutput) string {
	if candidate.CategoryID == uuid.Nil || candidate.CategoryID == candidate.RootCategoryID {
		return ""
	}
	return candidate.CategoryID.String()
}
