package onboarding

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type suggestBudgetSplitUseCase interface {
	Execute(ctx context.Context, in onbusecases.SuggestBudgetSplitInput) (onbusecases.SuggestBudgetSplitResult, error)
}

type BudgetSplitSuggester struct {
	uc suggestBudgetSplitUseCase
}

func NewBudgetSplitSuggester(uc suggestBudgetSplitUseCase) *BudgetSplitSuggester {
	return &BudgetSplitSuggester{uc: uc}
}

func (s *BudgetSplitSuggester) Suggest(ctx context.Context, userID uuid.UUID, objectiveProfile, objective string, incomeCents int64) ([]onbusecases.SuggestBudgetSplitView, error) {
	result, err := s.uc.Execute(ctx, onbusecases.SuggestBudgetSplitInput{
		UserID:           userID,
		ObjectiveProfile: objectiveProfile,
		Objective:        objective,
		IncomeCents:      incomeCents,
	})
	if err != nil {
		return nil, fmt.Errorf("agent.budget_split_suggester: %w", err)
	}
	return result.Splits, nil
}
