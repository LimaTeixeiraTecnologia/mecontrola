package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type kernelCategorySearchUC interface {
	Execute(ctx context.Context, in *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error)
}

func NewKernelCategoryResolver(search kernelCategorySearchUC) steps.CategoryResolverFunc {
	return func(ctx context.Context, state steps.ExpenseState) (steps.ExpenseState, error) {
		hint := strings.TrimSpace(state.CategoryHint)
		if hint == "" {
			hint = strings.TrimSpace(state.Merchant)
		}
		if hint == "" && state.Kind != intent.KindRecordIncome {
			return state, tools.ErrCategoryHintMissing
		}
		if hint == "" {
			hint = "salário"
		}
		categoryKind := categoriesvo.KindExpense
		if state.Kind == intent.KindRecordIncome {
			categoryKind = categoriesvo.KindIncome
		}
		result, err := search.Execute(ctx, &categoriesinput.SearchDictionaryInput{Query: hint, Kind: categoryKind})
		if err != nil {
			return state, fmt.Errorf("kernel: resolve category: %w", err)
		}
		if result == nil || len(result.Candidates) == 0 {
			return state, tools.ErrCategoryNotFound
		}
		top := result.Candidates[0]
		if top.IsAmbiguous && len(result.Candidates) > 1 {
			return state, &tools.CategoryAmbiguousError{
				Hint:       hint,
				Candidates: kernelCandidatePaths(result.Candidates),
			}
		}
		if top.Score >= categoriesvo.ScoreAutoThreshold || kernelIsExactMatch(top) {
			state.CategoryID = top.Path
			state.CategoryPath = top.Path
			return state, nil
		}
		if top.Score >= categoriesvo.ScoreConfirmThreshold {
			return state, &tools.CategoryNeedsConfirmationError{
				Hint:       hint,
				Candidates: kernelCandidatePaths(result.Candidates),
			}
		}
		return state, tools.ErrCategoryNotFound
	}
}

func NewKernelPersistFunc(expenseRecorder tools.ExpenseRecorder, cardPurchaseLog tools.CardPurchaseLogger) steps.PersistFunc {
	return func(ctx context.Context, state steps.ExpenseState) (steps.PersistResult, error) {
		if state.Kind == intent.KindRecordCardPurchase {
			return kernelPersistCardPurchase(ctx, state, cardPurchaseLog)
		}
		return kernelPersistExpense(ctx, state, expenseRecorder)
	}
}

func kernelPersistExpense(ctx context.Context, state steps.ExpenseState, recorder tools.ExpenseRecorder) (steps.PersistResult, error) {
	cat := state.CategoryID
	forceCat := &cat
	if state.ForceCategory != nil && strings.TrimSpace(*state.ForceCategory) != "" {
		forceCat = state.ForceCategory
	}
	result, err := recorder.Execute(ctx, tools.ExpenseRecorderInput{
		UserID:        state.UserID.String(),
		ForceCategory: forceCat,
		AmountCents:   state.AmountCents,
		Merchant:      state.Merchant,
		PaymentMethod: state.PaymentMethod,
		Direction:     state.Direction,
		OccurredAt:    state.OccurredAt,
	})
	if err != nil {
		return steps.PersistResult{}, fmt.Errorf("kernel: persist expense: %w", err)
	}
	return steps.PersistResult{
		AmountCents:  result.AmountCents,
		CategoryPath: result.CategoryPath,
	}, nil
}

func kernelPersistCardPurchase(ctx context.Context, state steps.ExpenseState, logger tools.CardPurchaseLogger) (steps.PersistResult, error) {
	if logger == nil {
		return steps.PersistResult{}, fmt.Errorf("kernel: persist card purchase: logger not configured")
	}
	cat := state.CategoryID
	forceCat := &cat
	if state.ForceCategory != nil && strings.TrimSpace(*state.ForceCategory) != "" {
		forceCat = state.ForceCategory
	}
	result, err := logger.Execute(ctx, tools.CardPurchaseLoggerInput{
		UserID:        state.UserID.String(),
		ForceCategory: forceCat,
		AmountCents:   state.AmountCents,
		Merchant:      state.Merchant,
		PaymentMethod: state.PaymentMethod,
		CardHint:      state.CardHint,
		Installments:  state.Installments,
	})
	if err != nil {
		return steps.PersistResult{}, fmt.Errorf("kernel: persist card purchase: %w", err)
	}
	return steps.PersistResult{
		AmountCents:  result.AmountCents,
		CategoryPath: result.CategoryPath,
		CardFound:    result.CardFound,
		CardName:     result.CardName,
	}, nil
}

func kernelCandidatePaths(candidates []categoriesoutput.CandidateOutput) []string {
	limit := 3
	if len(candidates) < limit {
		limit = len(candidates)
	}
	paths := make([]string, 0, limit)
	for _, c := range candidates[:limit] {
		path := strings.TrimSpace(c.Path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func kernelIsExactMatch(c categoriesoutput.CandidateOutput) bool {
	return c.MatchQuality == categoriesvo.MatchQualityExact.String() &&
		c.Confidence == categoriesvo.ConfidenceHigh.String()
}
