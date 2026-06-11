package mappers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type BudgetMapperSuite struct {
	suite.Suite
}

func TestBudgetMapperSuite(t *testing.T) {
	suite.Run(t, new(BudgetMapperSuite))
}

func (s *BudgetMapperSuite) TestBudget() {
	id := uuid.New()
	userID := uuid.New()
	competence, err := valueobjects.NewCompetence("2026-06")
	s.Require().NoError(err)
	rootSlug, err := valueobjects.ParseRootSlug("expense.custo_fixo")
	s.Require().NoError(err)
	createdAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	activatedAt := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)

	allocs := []entities.Allocation{
		entities.NewAllocation(id, rootSlug, 10000, 50000),
	}

	tests := []struct {
		name      string
		budget    entities.Budget
		wantState string
		wantActAt *time.Time
	}{
		{
			name: "budget em draft",
			budget: entities.HydrateBudget(
				id, userID, competence, 50000, entities.BudgetStateDraft,
				nil, false, allocs, createdAt, updatedAt,
			),
			wantState: "draft",
		},
		{
			name: "budget ativo",
			budget: entities.HydrateBudget(
				id, userID, competence, 50000, entities.BudgetStateActive,
				&activatedAt, false, allocs, createdAt, updatedAt,
			),
			wantState: "active",
			wantActAt: &activatedAt,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			out := mappers.M.Budget(tt.budget)
			s.Equal(id.String(), out.ID)
			s.Equal(userID.String(), out.UserID)
			s.Equal("2026-06", out.Competence)
			s.Equal(int64(50000), out.TotalCents)
			s.Equal(tt.wantState, out.State)
			s.False(out.AutoDraft)
			s.Equal(tt.wantActAt, out.ActivatedAt)
			s.Equal(createdAt, out.CreatedAt)
			s.Equal(updatedAt, out.UpdatedAt)
			s.Len(out.Allocations, 1)
			s.Equal("expense.custo_fixo", out.Allocations[0].RootSlug)
			s.Equal(10000, out.Allocations[0].BasisPoints)
			s.Equal(int64(50000), out.Allocations[0].PlannedCents)
		})
	}
}

func (s *BudgetMapperSuite) TestBudgets() {
	id := uuid.New()
	userID := uuid.New()
	competence, err := valueobjects.NewCompetence("2026-06")
	s.Require().NoError(err)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	bs := []entities.Budget{
		entities.HydrateBudget(id, userID, competence, 1000, entities.BudgetStateDraft, nil, false, nil, now, now),
		entities.HydrateBudget(uuid.New(), userID, competence, 2000, entities.BudgetStateActive, &now, true, nil, now, now),
	}
	outs := mappers.M.Budgets(bs)
	s.Len(outs, 2)
	s.Equal("draft", outs[0].State)
	s.Equal("active", outs[1].State)
	s.True(outs[1].AutoDraft)
}
