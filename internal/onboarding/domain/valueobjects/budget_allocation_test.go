package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func amountsForTotal() []valueobjects.CategoryAmount {
	return []valueobjects.CategoryAmount{
		{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 200000},
		{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 50000},
		{Kind: valueobjects.CategoryKindPleasures, AmountCents: 75000},
		{Kind: valueobjects.CategoryKindGoals, AmountCents: 100000},
		{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 75000},
	}
}

func TestNewBudgetAllocationFromAmounts_HappyPath(t *testing.T) {
	t.Parallel()
	got, err := valueobjects.NewBudgetAllocationFromAmounts(amountsForTotal(), 500000)
	require.NoError(t, err)
	require.Equal(t, 40, got.Percent(valueobjects.CategoryKindFixedCost))
	require.Equal(t, 10, got.Percent(valueobjects.CategoryKindKnowledge))
	require.Equal(t, 15, got.Percent(valueobjects.CategoryKindPleasures))
	require.Equal(t, 20, got.Percent(valueobjects.CategoryKindGoals))
	require.Equal(t, 15, got.Percent(valueobjects.CategoryKindFinancialFreedom))

	sum := 0
	for _, a := range got.Allocations() {
		sum += a.BasisPoints
	}
	require.Equal(t, 10000, sum)
}

func TestNewBudgetAllocationFromAmounts_SumAbove(t *testing.T) {
	t.Parallel()
	items := amountsForTotal()
	items[0].AmountCents = 300000
	_, err := valueobjects.NewBudgetAllocationFromAmounts(items, 500000)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrBudgetAllocationSumMismatch))
}

func TestNewBudgetAllocationFromAmounts_SumBelow(t *testing.T) {
	t.Parallel()
	items := amountsForTotal()
	items[0].AmountCents = 100000
	_, err := valueobjects.NewBudgetAllocationFromAmounts(items, 500000)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrBudgetAllocationSumMismatch))
}

func TestNewBudgetAllocationFromAmounts_WrongSize(t *testing.T) {
	t.Parallel()
	_, err := valueobjects.NewBudgetAllocationFromAmounts(amountsForTotal()[:4], 500000)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrBudgetAllocationWrongSize))
}

func TestNewBudgetAllocationFromAmounts_Duplicated(t *testing.T) {
	t.Parallel()
	items := amountsForTotal()
	items[1].Kind = valueobjects.CategoryKindFixedCost
	_, err := valueobjects.NewBudgetAllocationFromAmounts(items, 500000)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrBudgetAllocationWrongSize))
}

func TestNewBudgetAllocationFromAmounts_NegativeAmount(t *testing.T) {
	t.Parallel()
	items := amountsForTotal()
	items[0].AmountCents = -1
	_, err := valueobjects.NewBudgetAllocationFromAmounts(items, 500000)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrBudgetAllocationOutOfRange))
}

func TestNewBudgetAllocationFromAmounts_NonRoundDistributionStillSums10000(t *testing.T) {
	t.Parallel()
	items := []valueobjects.CategoryAmount{
		{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 33333},
		{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 33333},
		{Kind: valueobjects.CategoryKindPleasures, AmountCents: 33334},
		{Kind: valueobjects.CategoryKindGoals, AmountCents: 0},
		{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 0},
	}
	got, err := valueobjects.NewBudgetAllocationFromAmounts(items, 100000)
	require.NoError(t, err)
	sum := 0
	for _, a := range got.Allocations() {
		sum += a.BasisPoints
	}
	require.Equal(t, 10000, sum)
}
