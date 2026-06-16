package valueobjects

import (
	"errors"
	"fmt"
)

var (
	ErrCategorySplitWrongSize  = errors.New("onboarding: category split must contain exactly 5 percentages")
	ErrCategorySplitOutOfRange = errors.New("onboarding: category split percentage out of range")
	ErrCategorySplitSumInvalid = errors.New("onboarding: category split sum must be 100 (±1)")
)

type CategoryKind uint8

const (
	CategoryKindFixedCost CategoryKind = iota + 1
	CategoryKindKnowledge
	CategoryKindPleasures
	CategoryKindGoals
	CategoryKindFinancialFreedom
)

func (k CategoryKind) String() string {
	switch k {
	case CategoryKindFixedCost:
		return "fixed_cost"
	case CategoryKindKnowledge:
		return "knowledge"
	case CategoryKindPleasures:
		return "pleasures"
	case CategoryKindGoals:
		return "goals"
	case CategoryKindFinancialFreedom:
		return "financial_freedom"
	default:
		return "unknown"
	}
}

type CategoryAllocation struct {
	Kind    CategoryKind
	Percent int
}

type CategorySplit struct {
	allocations [5]CategoryAllocation
}

func NewCategorySplit(allocations []CategoryAllocation) (CategorySplit, error) {
	if len(allocations) != 5 {
		return CategorySplit{}, fmt.Errorf("onboarding: got %d: %w", len(allocations), ErrCategorySplitWrongSize)
	}
	sum := 0
	seen := make(map[CategoryKind]struct{}, 5)
	var fixed [5]CategoryAllocation
	for i, a := range allocations {
		if a.Percent < 0 || a.Percent > 100 {
			return CategorySplit{}, fmt.Errorf("onboarding: kind=%s percent=%d: %w", a.Kind.String(), a.Percent, ErrCategorySplitOutOfRange)
		}
		if a.Kind < CategoryKindFixedCost || a.Kind > CategoryKindFinancialFreedom {
			return CategorySplit{}, fmt.Errorf("onboarding: kind=%d: %w", a.Kind, ErrCategorySplitOutOfRange)
		}
		if _, dup := seen[a.Kind]; dup {
			return CategorySplit{}, fmt.Errorf("onboarding: duplicated kind=%s: %w", a.Kind.String(), ErrCategorySplitWrongSize)
		}
		seen[a.Kind] = struct{}{}
		fixed[i] = a
		sum += a.Percent
	}
	if sum < 99 || sum > 101 {
		return CategorySplit{}, fmt.Errorf("onboarding: sum=%d: %w", sum, ErrCategorySplitSumInvalid)
	}
	return CategorySplit{allocations: fixed}, nil
}

func (c CategorySplit) Allocations() []CategoryAllocation {
	out := make([]CategoryAllocation, 5)
	copy(out, c.allocations[:])
	return out
}

func (c CategorySplit) Percent(kind CategoryKind) int {
	for _, a := range c.allocations {
		if a.Kind == kind {
			return a.Percent
		}
	}
	return 0
}
