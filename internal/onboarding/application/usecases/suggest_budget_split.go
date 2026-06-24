package usecases

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	budgetusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	onbvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type SuggestBudgetSplitInput struct {
	UserID           uuid.UUID
	ObjectiveProfile string
	Objective        string
	IncomeCents      int64
}

type SuggestBudgetSplitView struct {
	RootSlug     string
	BasisPoints  int
	PlannedCents int64
}

type SuggestBudgetSplitResult struct {
	Splits []SuggestBudgetSplitView
}

type BudgetAllocator interface {
	Suggest(ctx context.Context, totalCents int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error)
}

type SuggestBudgetSplit struct {
	allocator BudgetAllocator
	o11y      observability.Observability
}

func NewSuggestBudgetSplit(allocator BudgetAllocator, o11y observability.Observability) *SuggestBudgetSplit {
	return &SuggestBudgetSplit{allocator: allocator, o11y: o11y}
}

func (uc *SuggestBudgetSplit) Execute(ctx context.Context, in SuggestBudgetSplitInput) (SuggestBudgetSplitResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.suggest_budget_split")
	defer span.End()

	if in.UserID == uuid.Nil {
		return SuggestBudgetSplitResult{}, fmt.Errorf("onboarding: suggest_budget_split: user_id required")
	}
	if in.IncomeCents <= 0 {
		return SuggestBudgetSplitResult{}, fmt.Errorf("onboarding: suggest_budget_split: income_cents must be greater than zero")
	}

	profile := resolveProfile(in.ObjectiveProfile, in.Objective)
	template := onbvo.SplitTemplate(profile)

	bp := make([]budgetusecases.AllocationBP, len(template))
	for i, e := range template {
		bp[i] = budgetusecases.AllocationBP{RootSlug: e.RootSlug, BasisPoints: e.BasisPoints}
	}

	allocations, err := uc.allocator.Suggest(ctx, in.IncomeCents, bp)
	if err != nil {
		return SuggestBudgetSplitResult{}, fmt.Errorf("onboarding: suggest_budget_split: allocate: %w", err)
	}

	splits := make([]SuggestBudgetSplitView, len(allocations))
	for i, a := range allocations {
		splits[i] = SuggestBudgetSplitView{
			RootSlug:     a.RootSlug,
			BasisPoints:  a.BasisPoints,
			PlannedCents: a.PlannedCents,
		}
	}
	return SuggestBudgetSplitResult{Splits: splits}, nil
}

func resolveProfile(hint, objective string) onbvo.ObjectiveProfile {
	return onbvo.ResolveProfile(hint, objective)
}
