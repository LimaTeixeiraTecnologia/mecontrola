package money_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
)

func TestMoney_BRLFormat(t *testing.T) {
	t.Parallel()
	require.Equal(t, "R$ 5.000,00", money.FromCents(500000).BRL())
	require.Equal(t, "R$ 35,00", money.FromCents(3500).BRL())
	require.Equal(t, "R$ 1.234.567,89", money.FromCents(123456789).BRL())
	require.Equal(t, "R$ 0,00", money.FromCents(0).BRL())
	require.Equal(t, "R$ 1.000,00", money.FromCents(-100000).BRL())
}

func TestMoney_Amount(t *testing.T) {
	t.Parallel()
	require.Equal(t, "2.000,00", money.FromCents(200000).Amount())
	require.Equal(t, "500,00", money.FromCents(50000).Amount())
}

func TestMoney_ApplyBasisPoints(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(540000), money.FromCents(1350000).ApplyBasisPoints(4000).Cents())
	require.Equal(t, int64(135000), money.FromCents(1350000).ApplyBasisPoints(1000).Cents())
}

func TestMoney_BasisPointsOf(t *testing.T) {
	t.Parallel()
	require.Equal(t, 4000, money.FromCents(200000).BasisPointsOf(money.FromCents(500000)))
	require.Equal(t, 0, money.FromCents(100).BasisPointsOf(money.FromCents(0)))
}

func TestRoundHalfEvenDiv(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(2), money.RoundHalfEvenDiv(5, 2))
	require.Equal(t, int64(2), money.RoundHalfEvenDiv(3, 2))
	require.Equal(t, int64(4), money.RoundHalfEvenDiv(7, 2))
	require.Equal(t, int64(3), money.RoundHalfEvenDiv(6, 2))
	require.Equal(t, int64(1), money.RoundHalfEvenDiv(4, 3))
	require.Equal(t, int64(-2), money.RoundHalfEvenDiv(-5, 2))
	require.Equal(t, int64(0), money.RoundHalfEvenDiv(5, 0))
}
