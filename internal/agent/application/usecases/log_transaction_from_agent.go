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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var (
	ErrLogTransactionInvalidIntent             = errors.New("agent: log transaction: intent invalido")
	ErrLogTransactionNoCategoryHint            = errors.New("agent: log transaction: sem hint de categoria")
	ErrLogTransactionCategoryAmbiguous         = errors.New("agent: log transaction: categoria ambigua")
	ErrLogTransactionCategoryNeedsConfirmation = errors.New("agent: log transaction: categoria precisa de confirmacao")
	ErrLogTransactionCategoryNotFound          = errors.New("agent: log transaction: categoria nao encontrada")
)

type CategoryAmbiguousError struct {
	Hint          string
	Candidates    []string
	CandidateRefs []pendingexpense.CandidateRef
}

func (e *CategoryAmbiguousError) Error() string {
	return fmt.Sprintf("%s: hint=%q candidatos=%s", ErrLogTransactionCategoryAmbiguous.Error(), e.Hint, strings.Join(e.Candidates, ", "))
}

func (e *CategoryAmbiguousError) Unwrap() error {
	return ErrLogTransactionCategoryAmbiguous
}

type CategoryNeedsConfirmationError struct {
	Hint          string
	Candidates    []string
	CandidateRefs []pendingexpense.CandidateRef
}

func (e *CategoryNeedsConfirmationError) Error() string {
	return fmt.Sprintf("%s: hint=%q candidatos=%s", ErrLogTransactionCategoryNeedsConfirmation.Error(), e.Hint, strings.Join(e.Candidates, ", "))
}

func (e *CategoryNeedsConfirmationError) Unwrap() error {
	return ErrLogTransactionCategoryNeedsConfirmation
}

func candidatePathsAndRefs(candidates []categoriesoutput.CandidateOutput) ([]string, []pendingexpense.CandidateRef) {
	limit := min(len(candidates), 3)
	paths := make([]string, 0, limit)
	refs := make([]pendingexpense.CandidateRef, 0, limit)
	for _, candidate := range candidates[:limit] {
		path := strings.TrimSpace(candidate.Path)
		if path == "" {
			continue
		}
		paths = append(paths, path)
		sub := ""
		if subID := candidateSubcategoryUUID(candidate); subID != nil {
			sub = subID.String()
		}
		refs = append(refs, pendingexpense.CandidateRef{
			RootCategoryID: candidate.RootCategoryID.String(),
			SubcategoryID:  sub,
		})
	}
	return paths, refs
}

func newCategoryAmbiguousError(hint string, candidates []categoriesoutput.CandidateOutput) *CategoryAmbiguousError {
	paths, refs := candidatePathsAndRefs(candidates)
	return &CategoryAmbiguousError{Hint: strings.TrimSpace(hint), Candidates: paths, CandidateRefs: refs}
}

func newCategoryNeedsConfirmationError(hint string, candidates []categoriesoutput.CandidateOutput) *CategoryNeedsConfirmationError {
	paths, refs := candidatePathsAndRefs(candidates)
	return &CategoryNeedsConfirmationError{Hint: strings.TrimSpace(hint), Candidates: paths, CandidateRefs: refs}
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

type RecordTransactionFromAgent struct {
	resolver       CategoryResolver
	creator        TransactionCreator
	o11y           observability.Observability
	persisted      observability.Counter
	resolveBad     observability.Counter
	scoreHistogram observability.Histogram
}

func NewRecordTransactionFromAgent(
	resolver CategoryResolver,
	creator TransactionCreator,
	o11y observability.Observability,
) *RecordTransactionFromAgent {
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
	return &RecordTransactionFromAgent{
		resolver:       resolver,
		creator:        creator,
		o11y:           o11y,
		persisted:      persisted,
		resolveBad:     resolveBad,
		scoreHistogram: newMatchScoreHistogram(o11y),
	}
}

type RecordTransactionFromAgentInput struct {
	UserID           string
	Intent           intent.Intent
	ForceCategory    *string
	ForceSubcategory *string
	AmountCents      int64
	Merchant         string
	PaymentMethod    string
	Direction        string
	OccurredAt       string
}

type RecordTransactionFromAgentResult struct {
	Persisted    bool
	AmountCents  int64
	Direction    string
	CategoryPath string
	OccurredAt   time.Time
}

func (uc *RecordTransactionFromAgent) Execute(ctx context.Context, in RecordTransactionFromAgentInput) (RecordTransactionFromAgentResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.log_transaction_from_agent")
	defer span.End()

	if strings.TrimSpace(in.UserID) == "" {
		return RecordTransactionFromAgentResult{}, errors.New("agent: log transaction: user id vazio")
	}

	if in.ForceCategory != nil && strings.TrimSpace(*in.ForceCategory) != "" {
		return uc.executeForced(ctx, in)
	}

	direction, categoryKind, err := directionForKind(in.Intent.Kind())
	if err != nil {
		return RecordTransactionFromAgentResult{}, err
	}
	if in.Intent.AmountCents() <= 0 {
		return RecordTransactionFromAgentResult{}, errors.New("agent: log transaction: amount invalido")
	}

	hint := strings.TrimSpace(in.Intent.CategoryHint())
	if hint == "" {
		hint = strings.TrimSpace(in.Intent.Merchant())
	}
	if hint == "" {
		if direction != directionIncome {
			uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_hint"))
			return RecordTransactionFromAgentResult{}, ErrLogTransactionNoCategoryHint
		}
		hint = defaultIncomeHint
	}

	candidate, path, err := uc.resolve(ctx, hint, categoryKind)
	if err != nil {
		return RecordTransactionFromAgentResult{}, err
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
		return RecordTransactionFromAgentResult{}, fmt.Errorf("agent: log transaction: create: %w", err)
	}

	uc.persisted.Add(ctx, 1, observability.String("direction", direction))
	return RecordTransactionFromAgentResult{
		Persisted:    true,
		AmountCents:  result.AmountCents,
		Direction:    result.Direction,
		CategoryPath: path,
		OccurredAt:   now,
	}, nil
}

func (uc *RecordTransactionFromAgent) executeForced(ctx context.Context, in RecordTransactionFromAgentInput) (RecordTransactionFromAgentResult, error) {
	forcedPath := strings.TrimSpace(*in.ForceCategory)
	description := strings.TrimSpace(in.Merchant)
	if description == "" {
		description = forcedPath
	}
	direction := strings.TrimSpace(in.Direction)
	if direction == "" {
		direction = directionOutcome
	}
	paymentMethod := mapPaymentMethod(in.PaymentMethod, direction)
	now := time.Now().UTC()
	result, err := uc.creator.Execute(ctx, CreateTransactionCommand{
		UserID:         in.UserID,
		Direction:      direction,
		PaymentMethod:  paymentMethod,
		Description:    description,
		RootCategoryID: forcedPath,
		SubcategoryID:  forcedSubcategory(in.ForceSubcategory),
		AmountCents:    in.AmountCents,
		OccurredAt:     now,
	})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "force_category_create_failed"))
		return RecordTransactionFromAgentResult{}, fmt.Errorf("agent: log transaction: force category create: %w", err)
	}
	uc.persisted.Add(ctx, 1, observability.String("direction", direction))
	return RecordTransactionFromAgentResult{
		Persisted:    true,
		AmountCents:  result.AmountCents,
		Direction:    result.Direction,
		CategoryPath: forcedPath,
		OccurredAt:   now,
	}, nil
}

func (uc *RecordTransactionFromAgent) resolve(ctx context.Context, hint string, kind categoriesvo.Kind) (categoriesoutput.CandidateOutput, string, error) {
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
		recordMatchScore(ctx, uc.scoreHistogram, top.Score, "ambiguous")
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "ambiguous"))
		return categoriesoutput.CandidateOutput{}, "", newCategoryAmbiguousError(hint, result.Candidates)
	}
	switch {
	case top.Score >= categoriesvo.ScoreAutoThreshold || isUnequivocalExactMatch(top):
		recordMatchScore(ctx, uc.scoreHistogram, top.Score, "auto_logged")
		return top, top.Path, nil
	case top.Score >= categoriesvo.ScoreConfirmThreshold:
		recordMatchScore(ctx, uc.scoreHistogram, top.Score, "needs_confirmation")
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "needs_confirmation"))
		return categoriesoutput.CandidateOutput{}, "", newCategoryNeedsConfirmationError(hint, result.Candidates)
	default:
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "low_score"))
		return categoriesoutput.CandidateOutput{}, "", ErrLogTransactionCategoryNotFound
	}
}

func directionForKind(kind intent.Kind) (string, categoriesvo.Kind, error) {
	switch kind {
	case intent.KindRecordExpense:
		return directionOutcome, categoriesvo.KindExpense, nil
	case intent.KindRecordIncome:
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

func forcedSubcategory(sub *string) string {
	if sub == nil {
		return ""
	}
	return strings.TrimSpace(*sub)
}
