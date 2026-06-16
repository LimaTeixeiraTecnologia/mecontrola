package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func defaultSplit() []valueobjects.CategoryAllocation {
	return []valueobjects.CategoryAllocation{
		{Kind: valueobjects.CategoryKindFixedCost, Percent: 40},
		{Kind: valueobjects.CategoryKindKnowledge, Percent: 10},
		{Kind: valueobjects.CategoryKindPleasures, Percent: 15},
		{Kind: valueobjects.CategoryKindGoals, Percent: 20},
		{Kind: valueobjects.CategoryKindFinancialFreedom, Percent: 15},
	}
}

func TestNewCategorySplit_HappyPath(t *testing.T) {
	t.Parallel()
	got, err := valueobjects.NewCategorySplit(defaultSplit())
	require.NoError(t, err)
	require.Equal(t, 40, got.Percent(valueobjects.CategoryKindFixedCost))
	require.Equal(t, 15, got.Percent(valueobjects.CategoryKindFinancialFreedom))
	require.Len(t, got.Allocations(), 5)
}

func TestNewCategorySplit_ToleranceWithinOne(t *testing.T) {
	t.Parallel()
	alloc := defaultSplit()
	alloc[0].Percent = 39
	_, err := valueobjects.NewCategorySplit(alloc)
	require.NoError(t, err)
}

func TestNewCategorySplit_SumTooLow(t *testing.T) {
	t.Parallel()
	alloc := defaultSplit()
	alloc[0].Percent = 30
	_, err := valueobjects.NewCategorySplit(alloc)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrCategorySplitSumInvalid))
}

func TestNewCategorySplit_WrongSize(t *testing.T) {
	t.Parallel()
	_, err := valueobjects.NewCategorySplit(defaultSplit()[:4])
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrCategorySplitWrongSize))
}

func TestNewCategorySplit_Duplicated(t *testing.T) {
	t.Parallel()
	alloc := defaultSplit()
	alloc[1].Kind = valueobjects.CategoryKindFixedCost
	_, err := valueobjects.NewCategorySplit(alloc)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrCategorySplitWrongSize))
}

func TestNewCategorySplit_NegativePercent(t *testing.T) {
	t.Parallel()
	alloc := defaultSplit()
	alloc[0].Percent = -1
	_, err := valueobjects.NewCategorySplit(alloc)
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrCategorySplitOutOfRange))
}
