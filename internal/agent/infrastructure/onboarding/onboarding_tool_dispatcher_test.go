package onboarding

import (
	"testing"

	"github.com/stretchr/testify/require"

	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func TestFormatReaisCents(t *testing.T) {
	t.Parallel()
	require.Equal(t, "5.000,00", formatReaisCents(500000))
	require.Equal(t, "35,00", formatReaisCents(3500))
	require.Equal(t, "1.234,56", formatReaisCents(123456))
}

func TestFormatReaisCents_HalfEvenBudgetSplit(t *testing.T) {
	t.Parallel()
	require.Equal(t, "5.000,00", formatReaisCents(500000))
	require.Equal(t, "0,00", formatReaisCents(0))
}

func TestSplitsSuccessReply_MatchesRunbook(t *testing.T) {
	t.Parallel()
	views := []onbusecases.OnboardingSplitView{
		{Kind: onbvalueobjects.CategoryKindFinancialFreedom, Percent: 15, AmountCents: 75000},
		{Kind: onbvalueobjects.CategoryKindFixedCost, Percent: 40, AmountCents: 200000},
		{Kind: onbvalueobjects.CategoryKindGoals, Percent: 20, AmountCents: 100000},
		{Kind: onbvalueobjects.CategoryKindKnowledge, Percent: 10, AmountCents: 50000},
		{Kind: onbvalueobjects.CategoryKindPleasures, Percent: 15, AmountCents: 75000},
	}
	got := splitsSuccessReply(views)
	require.Contains(t, got, "✅ *Distribuição salva!*")
	require.Contains(t, got, "💰 Custo Fixo: R$ 2.000,00 (40%)")
	require.Contains(t, got, "🎓 Conhecimento: R$ 500,00 (10%)")
	require.Contains(t, got, "🎉 Prazeres: R$ 750,00 (15%)")
	require.Contains(t, got, "🎯 Metas: R$ 1.000,00 (20%)")
	require.Contains(t, got, "🏦 Liberdade Financeira: R$ 750,00 (15%)")
	require.NotContains(t, got, "**")
}

func TestSplitsMismatchReply_Overflow(t *testing.T) {
	t.Parallel()
	got := splitsMismatchReply(600000, 500000)
	require.Contains(t, got, "passou *R$ 1.000,00*")
	require.Contains(t, got, "distribuiu *R$ 6.000,00*")
	require.Contains(t, got, "orçamento é *R$ 5.000,00*")
}

func TestSplitsMismatchReply_Underflow(t *testing.T) {
	t.Parallel()
	got := splitsMismatchReply(400000, 500000)
	require.Contains(t, got, "faltam *R$ 1.000,00*")
}

func TestParseAllocations(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"allocations": []any{
			map[string]any{"root_slug": "expense.custo_fixo", "amount_cents": float64(200000)},
			map[string]any{"root_slug": "expense.metas", "amount_cents": float64(100000)},
		},
	}
	items, ok := parseAllocations(args)
	require.True(t, ok)
	require.Len(t, items, 2)
	require.Equal(t, onbvalueobjects.CategoryKindFixedCost, items[0].Kind)
	require.Equal(t, int64(200000), items[0].AmountCents)
	require.Equal(t, onbvalueobjects.CategoryKindGoals, items[1].Kind)
}

func TestParseAllocations_UnknownSlug(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"allocations": []any{
			map[string]any{"root_slug": "expense.invalid", "amount_cents": float64(1)},
		},
	}
	_, ok := parseAllocations(args)
	require.False(t, ok)
}

func TestNumberToInt64(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(500000), numberToInt64(float64(500000)))
	require.Equal(t, int64(42), numberToInt64("42"))
	require.Equal(t, int64(0), numberToInt64(nil))
}
