package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AllocationInput struct {
	RootSlug    valueobjects.RootSlug
	BasisPoints int
}

type AllocationResult struct {
	RootSlug     valueobjects.RootSlug
	BasisPoints  int
	PlannedCents int64
}

type AllocationDistributor struct{}

func (AllocationDistributor) Distribute(totalCents int64, inputs []AllocationInput) []AllocationResult {
	results := make([]AllocationResult, len(inputs))
	var sumRaw int64
	for i, inp := range inputs {
		raw := totalCents * int64(inp.BasisPoints)
		quotient := raw / 10000
		remainder := raw % 10000
		planned := halfEvenRound(quotient, remainder)
		results[i] = AllocationResult{
			RootSlug:     inp.RootSlug,
			BasisPoints:  inp.BasisPoints,
			PlannedCents: planned,
		}
		sumRaw += planned
	}

	diff := totalCents - sumRaw
	if diff == 0 {
		return results
	}

	order := valueobjects.CanonicalOrder()

	if diff > 0 {
		distributed := int64(0)
		for _, slug := range order {
			if distributed >= diff {
				break
			}
			for i := range results {
				if results[i].RootSlug == slug {
					results[i].PlannedCents++
					distributed++
					break
				}
			}
		}
		return results
	}

	distributed := int64(0)
	abs := -diff
	for _, slug := range order {
		if distributed >= abs {
			break
		}
		for i := range results {
			if results[i].RootSlug == slug {
				results[i].PlannedCents--
				distributed++
				break
			}
		}
	}
	return results
}

func halfEvenRound(quotient, remainder int64) int64 {
	if remainder == 0 {
		return quotient
	}
	half := int64(5000)
	if remainder > half {
		return quotient + 1
	}
	if remainder < half {
		return quotient
	}
	if quotient%2 == 0 {
		return quotient
	}
	return quotient + 1
}
