package tools

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
)

func TestBudgetRootSlugForCategory(t *testing.T) {
	scenarios := []struct {
		name        string
		catalogSlug string
		expected    string
	}{
		{name: "prazeres sem hifen", catalogSlug: "prazeres", expected: "expense.prazeres"},
		{name: "custo fixo com hifen", catalogSlug: "custo-fixo", expected: "expense.custo_fixo"},
		{name: "conhecimento", catalogSlug: "conhecimento", expected: "expense.conhecimento"},
		{name: "liberdade financeira", catalogSlug: "liberdade-financeira", expected: "expense.liberdade_financeira"},
		{name: "categoria de receita nao orcada mantem slug original", catalogSlug: "salario", expected: "salario"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			assert.Equal(t, scenario.expected, budgetRootSlugForCategory(scenario.catalogSlug))
		})
	}
}

func TestMatchAllocation_UsaSlugCompound(t *testing.T) {
	allocations := []interfaces.AllocationSummary{
		{RootSlug: "expense.prazeres", PlannedCents: int64Ptr(208116), SpentCents: 3000},
	}

	alloc := matchAllocation(allocations, budgetRootSlugForCategory("prazeres"))

	require.NotNil(t, alloc.PlannedCents)
	assert.Equal(t, int64(208116), *alloc.PlannedCents)
	assert.Equal(t, int64(3000), alloc.SpentCents)
}

func TestMatchAllocation_SemAlocacaoMantemZerado(t *testing.T) {
	allocations := []interfaces.AllocationSummary{
		{RootSlug: "expense.custo_fixo", PlannedCents: int64Ptr(624348), SpentCents: 5000},
	}

	alloc := matchAllocation(allocations, budgetRootSlugForCategory("metas"))

	assert.Nil(t, alloc.PlannedCents)
	assert.Equal(t, int64(0), alloc.SpentCents)
}

func TestCategoryDetailExec_ResumoPorCategoria_CenarioProducao(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	ledger := imocks.NewTransactionsLedger(t)
	reader := imocks.NewCategoriesReader(t)

	competence := currentCompetenceFallback()

	planner.EXPECT().GetMonthlySummary(mock.Anything, testUserID, competence).
		Return(interfaces.BudgetSummary{
			Competence: competence,
			State:      "active",
			Allocations: []interfaces.AllocationSummary{
				{RootSlug: "expense.prazeres", PlannedCents: int64Ptr(208116), SpentCents: 3000},
			},
		}, nil).Once()

	reader.EXPECT().ListCategories(mock.Anything, testUserID).
		Return([]interfaces.Category{
			{ID: uuid.MustParse("ac535261-4060-56ef-b2e8-57c8cc7032d1"), Slug: "prazeres", Name: "Prazeres"},
		}, nil).Once()

	ledger.EXPECT().ListMonthlyEntries(mock.Anything, testUserID, competence, "", categoryDetailEntriesLimit).
		Return([]interfaces.MonthlyEntry{
			{
				CategoryID:              "ac535261-4060-56ef-b2e8-57c8cc7032d1",
				SubcategoryNameSnapshot: "Cinema e Teatro",
				AmountCents:             3000,
			},
		}, nil).Once()

	exec := buildCategoryDetailExec(planner, ledger, reader)
	out, err := exec(identityCtx("w1", 0), CategoryDetailInput{Category: "prazeres"})
	require.NoError(t, err)

	assert.Equal(t, categoryDetailOutcomeOK, out.Outcome)
	assert.Contains(t, out.Message, "R$ 2.081,16")
	assert.Contains(t, out.Message, "R$ 30,00")
	assert.Contains(t, out.Message, "R$ 2.051,16")
	assert.NotContains(t, out.Message, "Planejado: —")
}

func TestCategoryNamesBySlug_ResumoGeralNomeAmigavel(t *testing.T) {
	names := categoryNamesBySlug([]interfaces.Category{
		{Slug: "prazeres", Name: "Prazeres"},
		{Slug: "custo-fixo", Name: "Custo Fixo"},
	})

	assert.Equal(t, "Prazeres", friendlyCategoryName(names, "expense.prazeres"))
	assert.Equal(t, "Custo Fixo", friendlyCategoryName(names, "expense.custo_fixo"))
}

func int64Ptr(v int64) *int64 {
	return &v
}
