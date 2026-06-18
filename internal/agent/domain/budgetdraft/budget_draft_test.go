package budgetdraft_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type BudgetDraftSuite struct {
	suite.Suite
}

func TestBudgetDraftSuite(t *testing.T) {
	suite.Run(t, new(BudgetDraftSuite))
}

func (s *BudgetDraftSuite) TestNewIsEmptyAndIncomplete() {
	draft := budgetdraft.New("2026-06")
	s.Equal(int64(0), draft.TotalCents())
	s.Equal("2026-06", draft.Competence())
	s.Equal(0, draft.SumBasisPoints())
	s.Equal(10000, draft.RemainingBasisPoints())
	s.False(draft.IsComplete())
	s.Len(draft.MissingSlugs(), 5)
}

func (s *BudgetDraftSuite) TestMergeSetsTotalAndAllocations() {
	draft := budgetdraft.New("2026-06")
	merged, err := draft.Merge(budgetdraft.Change{
		TotalCents: 500000,
		Allocations: map[string]int{
			budgetdraft.SlugCustoFixo: 3500,
		},
	})
	s.Require().NoError(err)
	s.Equal(int64(500000), merged.TotalCents())
	s.Equal(3500, merged.SumBasisPoints())
	s.Equal(6500, merged.RemainingBasisPoints())
	s.False(merged.IsComplete())
}

func (s *BudgetDraftSuite) TestMergeIsImmutable() {
	draft := budgetdraft.New("2026-06")
	_, err := draft.Merge(budgetdraft.Change{TotalCents: 500000})
	s.Require().NoError(err)
	s.Equal(int64(0), draft.TotalCents())
}

func (s *BudgetDraftSuite) TestMergeCompletesWhenSumIs10000AndTotalPositive() {
	draft := budgetdraft.New("2026-06")
	merged, err := draft.Merge(budgetdraft.Change{
		TotalCents: 800000,
		Allocations: map[string]int{
			budgetdraft.SlugCustoFixo:           3500,
			budgetdraft.SlugConhecimento:        1000,
			budgetdraft.SlugPrazeres:            2000,
			budgetdraft.SlugMetas:               2000,
			budgetdraft.SlugLiberdadeFinanceira: 1500,
		},
	})
	s.Require().NoError(err)
	s.True(merged.IsComplete())
	s.Empty(merged.MissingSlugs())
	s.Equal(0, merged.RemainingBasisPoints())
}

func (s *BudgetDraftSuite) TestMergeRejectsSlugOutsideAllowlist() {
	draft := budgetdraft.New("2026-06")
	_, err := draft.Merge(budgetdraft.Change{
		Allocations: map[string]int{"expense.viagem": 1000},
	})
	s.Require().ErrorIs(err, budgetdraft.ErrSlugNotAllowed)
}

func (s *BudgetDraftSuite) TestMergeRejectsBasisPointsOutOfRange() {
	draft := budgetdraft.New("2026-06")
	_, err := draft.Merge(budgetdraft.Change{
		Allocations: map[string]int{budgetdraft.SlugMetas: 20000},
	})
	s.Require().ErrorIs(err, budgetdraft.ErrBasisPointsRange)
}

func (s *BudgetDraftSuite) TestMergeRejectsNegativeTotal() {
	draft := budgetdraft.New("2026-06")
	_, err := draft.Merge(budgetdraft.Change{TotalCents: -1})
	s.Require().ErrorIs(err, budgetdraft.ErrTotalNegative)
}

func (s *BudgetDraftSuite) TestMergeKeepsExistingTotalWhenChangeTotalZero() {
	draft, err := budgetdraft.New("2026-06").Merge(budgetdraft.Change{TotalCents: 500000})
	s.Require().NoError(err)
	merged, err := draft.Merge(budgetdraft.Change{Allocations: map[string]int{budgetdraft.SlugMetas: 1000}})
	s.Require().NoError(err)
	s.Equal(int64(500000), merged.TotalCents())
}

func (s *BudgetDraftSuite) TestRestoreRoundTrip() {
	allocations := map[string]int{budgetdraft.SlugCustoFixo: 5000, budgetdraft.SlugPrazeres: 5000}
	draft, err := budgetdraft.Restore(700000, allocations, "2026-06")
	s.Require().NoError(err)
	s.Equal(int64(700000), draft.TotalCents())
	s.Equal(10000, draft.SumBasisPoints())
	s.True(draft.IsComplete())
}

func (s *BudgetDraftSuite) TestRestoreRejectsBadSlug() {
	_, err := budgetdraft.Restore(0, map[string]int{"x": 1000}, "2026-06")
	s.Require().ErrorIs(err, budgetdraft.ErrSlugNotAllowed)
}

func (s *BudgetDraftSuite) TestAllocationsReturnsCopy() {
	draft, err := budgetdraft.New("2026-06").Merge(budgetdraft.Change{Allocations: map[string]int{budgetdraft.SlugMetas: 1000}})
	s.Require().NoError(err)
	got := draft.Allocations()
	got[budgetdraft.SlugMetas] = 9999
	s.Equal(1000, draft.Allocations()[budgetdraft.SlugMetas])
}

func (s *BudgetDraftSuite) TestIsAllowedSlug() {
	s.True(budgetdraft.IsAllowedSlug(budgetdraft.SlugLiberdadeFinanceira))
	s.False(budgetdraft.IsAllowedSlug("expense.unknown"))
}
