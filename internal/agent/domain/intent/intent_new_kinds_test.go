package intent_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestNewKinds_StringAndParseRoundTrip(t *testing.T) {
	t.Parallel()

	kinds := []intent.Kind{
		intent.KindLogCardPurchase,
		intent.KindListTransactions,
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindCreateRecurring,
		intent.KindListRecurring,
	}
	for _, k := range kinds {
		parsed, err := intent.ParseKind(k.String())
		require.NoError(t, err)
		require.Equal(t, k, parsed)
	}
}

func TestNewLogCardPurchase(t *testing.T) {
	t.Parallel()

	t.Run("valido", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewLogCardPurchase(intent.LogCardPurchaseFields{
			AmountCents:  125090,
			Merchant:     "Magalu",
			CategoryHint: "Casa",
			CardHint:     "nubank",
			Installments: 12,
		})
		require.NoError(t, err)
		require.Equal(t, intent.KindLogCardPurchase, got.Kind())
		require.Equal(t, int64(125090), got.AmountCents())
		require.Equal(t, 12, got.Installments())
		require.Equal(t, "nubank", got.CardHint())
	})

	t.Run("amount nao positivo", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewLogCardPurchase(intent.LogCardPurchaseFields{AmountCents: 0, Installments: 3})
		require.ErrorIs(t, err, intent.ErrAmountNonPositive)
	})

	t.Run("parcelas minimas", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewLogCardPurchase(intent.LogCardPurchaseFields{AmountCents: 100, Installments: 1})
		require.ErrorIs(t, err, intent.ErrInstallmentsTooFew)
	})

	t.Run("parcelas excessivas", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewLogCardPurchase(intent.LogCardPurchaseFields{AmountCents: 100, Installments: 25})
		require.ErrorIs(t, err, intent.ErrInstallmentsTooMany)
	})
}

func TestNewListTransactions(t *testing.T) {
	t.Parallel()

	empty, err := intent.NewListTransactions("")
	require.NoError(t, err)
	require.Equal(t, intent.KindListTransactions, empty.Kind())
	require.Empty(t, empty.RefMonth())

	withMonth, err := intent.NewListTransactions("2026-05")
	require.NoError(t, err)
	require.Equal(t, "2026-05", withMonth.RefMonth())

	_, err = intent.NewListTransactions("2026/05")
	require.ErrorIs(t, err, intent.ErrRefMonthInvalid)
}

func TestNewDeleteLastTransaction(t *testing.T) {
	t.Parallel()
	got := intent.NewDeleteLastTransaction()
	require.Equal(t, intent.KindDeleteLastTransaction, got.Kind())
}

func TestNewEditLastTransaction(t *testing.T) {
	t.Parallel()

	got, err := intent.NewEditLastTransaction(8000)
	require.NoError(t, err)
	require.Equal(t, intent.KindEditLastTransaction, got.Kind())
	require.Equal(t, int64(8000), got.AmountCents())

	_, err = intent.NewEditLastTransaction(0)
	require.ErrorIs(t, err, intent.ErrAmountNonPositive)
}

func TestNewCreateRecurring(t *testing.T) {
	t.Parallel()

	t.Run("default frequency monthly", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
			AmountCents: 500000,
			Merchant:    "salário",
			Direction:   "income",
			DayOfMonth:  5,
		})
		require.NoError(t, err)
		require.Equal(t, intent.KindCreateRecurring, got.Kind())
		require.Equal(t, "income", got.Direction())
		require.Equal(t, "monthly", got.Frequency())
		require.Equal(t, 5, got.DayOfMonth())
	})

	t.Run("direction invalido", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewCreateRecurring(intent.CreateRecurringFields{AmountCents: 100, Direction: "x", DayOfMonth: 1})
		require.ErrorIs(t, err, intent.ErrDirectionInvalid)
	})

	t.Run("frequency invalida", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewCreateRecurring(intent.CreateRecurringFields{AmountCents: 100, Direction: "outcome", Frequency: "weekly", DayOfMonth: 1})
		require.ErrorIs(t, err, intent.ErrFrequencyInvalid)
	})

	t.Run("day_of_month invalido", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewCreateRecurring(intent.CreateRecurringFields{AmountCents: 100, Direction: "outcome", DayOfMonth: 32})
		require.ErrorIs(t, err, intent.ErrDayOfMonthInvalid)
	})

	t.Run("amount invalido", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewCreateRecurring(intent.CreateRecurringFields{AmountCents: 0, Direction: "income", DayOfMonth: 1})
		require.ErrorIs(t, err, intent.ErrAmountNonPositive)
	})
}

func TestNewListRecurring(t *testing.T) {
	t.Parallel()
	got := intent.NewListRecurring()
	require.Equal(t, intent.KindListRecurring, got.Kind())
}
