package binding

import (
	"context"
	"fmt"

	budgetusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
)

type BudgetAllocatorBinding struct {
	suggestAllocation *budgetusecases.SuggestAllocation
}

func NewBudgetAllocatorBinding(suggestAllocation *budgetusecases.SuggestAllocation) *BudgetAllocatorBinding {
	return &BudgetAllocatorBinding{suggestAllocation: suggestAllocation}
}

func (b *BudgetAllocatorBinding) Suggest(ctx context.Context, totalCents int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
	result, err := b.suggestAllocation.Execute(ctx, budgetusecases.SuggestAllocationInput{
		TotalCents:  totalCents,
		Allocations: bp,
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: budget_allocator: %w", err)
	}
	return result.Allocations, nil
}
