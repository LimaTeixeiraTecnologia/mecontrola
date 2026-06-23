package services

import (
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrCategoryPercentageTargetNotFound = errors.New("budgets: categoria alvo não encontrada nas allocations do orçamento")

var ErrCategoryPercentageNoAllocations = errors.New("budgets: orçamento sem allocations para rebalancear")

var ErrCategoryPercentageSumInvalid = errors.New("budgets: soma das allocations atuais deve ser 10000 para rebalancear")

type CategoryPercentageInput struct {
	RootSlug    valueobjects.RootSlug
	BasisPoints int
}

func DecideEditCategoryPercentage(current []CategoryPercentageInput, target valueobjects.RootSlug, targetBasisPoints valueobjects.BasisPoints) ([]CategoryPercentageInput, error) {
	if len(current) == 0 {
		return nil, ErrCategoryPercentageNoAllocations
	}

	sum := 0
	foundTarget := false
	for _, a := range current {
		sum += a.BasisPoints
		if a.RootSlug == target {
			foundTarget = true
		}
	}
	if !foundTarget {
		return nil, fmt.Errorf("budgets: %q: %w", target.String(), ErrCategoryPercentageTargetNotFound)
	}
	if sum != 10000 {
		return nil, fmt.Errorf("budgets: soma=%d: %w", sum, ErrCategoryPercentageSumInvalid)
	}

	targetBP := targetBasisPoints.Int()
	remaining := 10000 - targetBP

	othersTotal := 0
	for _, a := range current {
		if a.RootSlug == target {
			continue
		}
		othersTotal += a.BasisPoints
	}

	result := make([]CategoryPercentageInput, len(current))
	assigned := 0
	for i, a := range current {
		if a.RootSlug == target {
			result[i] = CategoryPercentageInput{RootSlug: a.RootSlug, BasisPoints: targetBP}
			continue
		}
		var share int
		if othersTotal > 0 {
			raw := int64(remaining) * int64(a.BasisPoints)
			share = int(raw / int64(othersTotal))
		}
		result[i] = CategoryPercentageInput{RootSlug: a.RootSlug, BasisPoints: share}
		assigned += share
	}

	leftover := remaining - assigned
	if leftover != 0 {
		applyLeftover(result, target, leftover)
	}

	return result, nil
}

func applyLeftover(result []CategoryPercentageInput, target valueobjects.RootSlug, leftover int) {
	order := valueobjects.CanonicalOrder()
	if leftover > 0 {
		idx := largestOtherIndex(result, target, order)
		if idx >= 0 {
			result[idx].BasisPoints += leftover
			return
		}
		for i := range result {
			if result[i].RootSlug == target {
				result[i].BasisPoints += leftover
				return
			}
		}
		return
	}

	deficit := -leftover
	for deficit > 0 {
		idx := largestOtherIndex(result, target, order)
		if idx < 0 || result[idx].BasisPoints == 0 {
			break
		}
		result[idx].BasisPoints--
		deficit--
	}
	if deficit > 0 {
		for i := range result {
			if result[i].RootSlug == target && result[i].BasisPoints >= deficit {
				result[i].BasisPoints -= deficit
				return
			}
		}
	}
}

func largestOtherIndex(result []CategoryPercentageInput, target valueobjects.RootSlug, order [5]valueobjects.RootSlug) int {
	best := -1
	bestBP := -1
	bestRank := len(order)
	for i := range result {
		if result[i].RootSlug == target {
			continue
		}
		rank := canonicalRank(result[i].RootSlug, order)
		if result[i].BasisPoints > bestBP || (result[i].BasisPoints == bestBP && rank < bestRank) {
			best = i
			bestBP = result[i].BasisPoints
			bestRank = rank
		}
	}
	return best
}

func canonicalRank(slug valueobjects.RootSlug, order [5]valueobjects.RootSlug) int {
	for i := range order {
		if order[i] == slug {
			return i
		}
	}
	return len(order)
}
