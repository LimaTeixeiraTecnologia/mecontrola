package valueobjects

import (
	"errors"
	"fmt"
)

var (
	ErrBudgetAllocationWrongSize   = errors.New("onboarding: budget allocation must contain exactly 5 categories")
	ErrBudgetAllocationOutOfRange  = errors.New("onboarding: budget allocation amount out of range")
	ErrBudgetAllocationSumMismatch = errors.New("onboarding: budget allocation must sum to the monthly budget")
)

const budgetAllocationTotalBasisPoints = 10000

type CategoryAmount struct {
	Kind        CategoryKind
	AmountCents int64
}

type CategoryBasisPoints struct {
	Kind        CategoryKind
	BasisPoints int
}

type BudgetAllocation struct {
	allocations [5]CategoryBasisPoints
}

func NewBudgetAllocationFromAmounts(items []CategoryAmount, totalCents int64) (BudgetAllocation, error) {
	if len(items) != 5 {
		return BudgetAllocation{}, fmt.Errorf("onboarding: got %d: %w", len(items), ErrBudgetAllocationWrongSize)
	}
	if totalCents <= 0 {
		return BudgetAllocation{}, fmt.Errorf("onboarding: total=%d: %w", totalCents, ErrBudgetAllocationOutOfRange)
	}
	var sum int64
	seen := make(map[CategoryKind]struct{}, 5)
	for _, it := range items {
		if it.AmountCents < 0 {
			return BudgetAllocation{}, fmt.Errorf("onboarding: kind=%s amount=%d: %w", it.Kind.String(), it.AmountCents, ErrBudgetAllocationOutOfRange)
		}
		if it.Kind < CategoryKindFixedCost || it.Kind > CategoryKindFinancialFreedom {
			return BudgetAllocation{}, fmt.Errorf("onboarding: kind=%d: %w", it.Kind, ErrBudgetAllocationOutOfRange)
		}
		if _, dup := seen[it.Kind]; dup {
			return BudgetAllocation{}, fmt.Errorf("onboarding: duplicated kind=%s: %w", it.Kind.String(), ErrBudgetAllocationWrongSize)
		}
		seen[it.Kind] = struct{}{}
		sum += it.AmountCents
	}
	if sum != totalCents {
		return BudgetAllocation{}, fmt.Errorf("onboarding: sum=%d total=%d: %w", sum, totalCents, ErrBudgetAllocationSumMismatch)
	}

	var allocations [5]CategoryBasisPoints
	assigned := 0
	last := len(items) - 1
	for i, it := range items {
		if i == last {
			allocations[i] = CategoryBasisPoints{Kind: it.Kind, BasisPoints: budgetAllocationTotalBasisPoints - assigned}
			continue
		}
		bp := int(roundBasisPointsHalfEven(it.AmountCents*budgetAllocationTotalBasisPoints, totalCents))
		allocations[i] = CategoryBasisPoints{Kind: it.Kind, BasisPoints: bp}
		assigned += bp
	}
	return BudgetAllocation{allocations: allocations}, nil
}

func roundBasisPointsHalfEven(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	quotient := numerator / denominator
	remainder := numerator % denominator
	twice := remainder * 2
	if twice > denominator || (twice == denominator && quotient%2 == 1) {
		return quotient + 1
	}
	return quotient
}

func (b BudgetAllocation) Allocations() []CategoryBasisPoints {
	out := make([]CategoryBasisPoints, 5)
	copy(out, b.allocations[:])
	return out
}

func (b BudgetAllocation) Percent(kind CategoryKind) int {
	for _, a := range b.allocations {
		if a.Kind == kind {
			return a.BasisPoints / 100
		}
	}
	return 0
}
