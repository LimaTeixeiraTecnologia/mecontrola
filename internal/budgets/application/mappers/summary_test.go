package mappers_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
)

type SummaryMapperSuite struct {
	suite.Suite
}

func TestSummaryMapperSuite(t *testing.T) {
	suite.Run(t, new(SummaryMapperSuite))
}

func (s *SummaryMapperSuite) TestMonthlySummary() {
	total := int64(100000)
	totalPlanned := int64(80000)
	percentage := 75.0

	in := mappers.MonthlySummaryInput{
		UserID:     "user-1",
		Competence: "2026-06",
		TotalCents: &total,
		AutoDraft:  false,
		State:      "active",
		Allocations: []output.AllocationSummary{
			{RootSlug: "expense.custo_fixo", PlannedCents: &totalPlanned, SpentCents: 60000, PercentageSpent: &percentage},
		},
		TotalSpentCents:   60000,
		TotalPlannedCents: &totalPlanned,
		PercentageTotal:   &percentage,
	}

	out := mappers.M.MonthlySummary(in)
	s.Equal("user-1", out.UserID)
	s.Equal("2026-06", out.Competence)
	s.Equal(&total, out.TotalCents)
	s.False(out.AutoDraft)
	s.Equal("active", out.State)
	s.Len(out.Allocations, 1)
	s.Equal(int64(60000), out.TotalSpentCents)
	s.Equal(&totalPlanned, out.TotalPlannedCents)
	s.Equal(&percentage, out.PercentageTotal)
}

func (s *SummaryMapperSuite) TestMonthlySummary_AutoDraftNoTotals() {
	in := mappers.MonthlySummaryInput{
		UserID:          "user-1",
		Competence:      "2026-06",
		AutoDraft:       true,
		State:           "draft",
		Allocations:     []output.AllocationSummary{},
		TotalSpentCents: 0,
	}
	out := mappers.M.MonthlySummary(in)
	s.Nil(out.TotalCents)
	s.Nil(out.TotalPlannedCents)
	s.Nil(out.PercentageTotal)
	s.True(out.AutoDraft)
}
