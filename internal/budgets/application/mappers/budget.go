package mappers

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

func (Mapper) Budget(b entities.Budget) output.BudgetOutput {
	allocs := make([]output.AllocationOutput, 0, len(b.Allocations()))
	for _, a := range b.Allocations() {
		allocs = append(allocs, output.AllocationOutput{
			RootSlug:     a.RootSlug().String(),
			BasisPoints:  a.BasisPoints(),
			PlannedCents: a.PlannedCents(),
		})
	}
	state := "draft"
	if b.IsActive() {
		state = "active"
	}
	return output.BudgetOutput{
		ID:          b.ID().String(),
		UserID:      b.UserID().String(),
		Competence:  b.Competence().String(),
		TotalCents:  b.TotalCents(),
		State:       state,
		AutoDraft:   b.AutoDraft(),
		ActivatedAt: b.ActivatedAt(),
		Allocations: allocs,
		CreatedAt:   b.CreatedAt(),
		UpdatedAt:   b.UpdatedAt(),
	}
}

func (m Mapper) Budgets(bs []entities.Budget) []output.BudgetOutput {
	outs := make([]output.BudgetOutput, 0, len(bs))
	for _, b := range bs {
		outs = append(outs, m.Budget(b))
	}
	return outs
}
