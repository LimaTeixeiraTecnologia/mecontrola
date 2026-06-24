package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AllocationBP struct {
	RootSlug    string
	BasisPoints int
}

type AllocationCents struct {
	RootSlug     string
	BasisPoints  int
	PlannedCents int64
}

type SuggestAllocationInput struct {
	TotalCents  int64
	Allocations []AllocationBP
}

func (i *SuggestAllocationInput) Validate() error {
	var errs []error
	if i.TotalCents <= 0 {
		errs = append(errs, errors.New("total_cents: must be greater than zero"))
	}
	if len(i.Allocations) == 0 {
		errs = append(errs, errors.New("allocations: must not be empty"))
	}
	for _, a := range i.Allocations {
		if a.BasisPoints < 0 {
			errs = append(errs, fmt.Errorf("allocations[%s].basis_points: must not be negative", a.RootSlug))
		}
	}
	return errors.Join(errs...)
}

type SuggestAllocationResult struct {
	Allocations []AllocationCents
}

type SuggestAllocation struct {
	distributor services.AllocationDistributor
}

func NewSuggestAllocation() *SuggestAllocation {
	return &SuggestAllocation{distributor: services.AllocationDistributor{}}
}

func (uc *SuggestAllocation) Execute(_ context.Context, in SuggestAllocationInput) (SuggestAllocationResult, error) {
	if err := in.Validate(); err != nil {
		return SuggestAllocationResult{}, err
	}

	inputs := make([]services.AllocationInput, 0, len(in.Allocations))
	for _, a := range in.Allocations {
		slug, err := valueobjects.ParseRootSlug(a.RootSlug)
		if err != nil {
			return SuggestAllocationResult{}, fmt.Errorf("suggest_allocation: %w", err)
		}
		inputs = append(inputs, services.AllocationInput{
			RootSlug:    slug,
			BasisPoints: a.BasisPoints,
		})
	}

	raw := uc.distributor.Distribute(in.TotalCents, inputs)

	result := make([]AllocationCents, len(raw))
	for i, r := range raw {
		result[i] = AllocationCents{
			RootSlug:     r.RootSlug.String(),
			BasisPoints:  r.BasisPoints,
			PlannedCents: r.PlannedCents,
		}
	}

	return SuggestAllocationResult{Allocations: result}, nil
}
