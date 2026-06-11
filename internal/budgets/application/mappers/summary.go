package mappers

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type MonthlySummaryInput struct {
	UserID            string
	Competence        string
	TotalCents        *int64
	AutoDraft         bool
	State             string
	Allocations       []output.AllocationSummary
	TotalSpentCents   int64
	TotalPlannedCents *int64
	PercentageTotal   *float64
}

func (Mapper) MonthlySummary(in MonthlySummaryInput) output.MonthlySummaryOutput {
	return output.MonthlySummaryOutput{
		UserID:            in.UserID,
		Competence:        in.Competence,
		TotalCents:        in.TotalCents,
		AutoDraft:         in.AutoDraft,
		State:             in.State,
		Allocations:       in.Allocations,
		TotalSpentCents:   in.TotalSpentCents,
		TotalPlannedCents: in.TotalPlannedCents,
		PercentageTotal:   in.PercentageTotal,
	}
}
