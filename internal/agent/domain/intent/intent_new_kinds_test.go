package intent_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestNewKinds_StringAndParseRoundTrip(t *testing.T) {
	t.Parallel()

	kinds := []intent.Kind{
		intent.KindRecordCardPurchase,
		intent.KindListTransactions,
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindCreateRecurring,
		intent.KindListRecurring,
		intent.KindCreateCard,
		intent.KindCountCards,
	}
	for _, k := range kinds {
		parsed, err := intent.ParseKind(k.String())
		require.NoError(t, err)
		require.Equal(t, k, parsed)
	}
}

func TestNewRecordCardPurchase(t *testing.T) {
	t.Parallel()

	t.Run("valido", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{
			AmountCents:  125090,
			Merchant:     "Magalu",
			CategoryHint: "Casa",
			CardHint:     "nubank",
			Installments: 12,
		})
		require.NoError(t, err)
		require.Equal(t, intent.KindRecordCardPurchase, got.Kind())
		require.Equal(t, int64(125090), got.AmountCents())
		require.Equal(t, 12, got.Installments())
		require.Equal(t, "nubank", got.CardHint())
	})

	t.Run("amount nao positivo", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{AmountCents: 0, Installments: 3})
		require.ErrorIs(t, err, intent.ErrAmountNonPositive)
	})

	t.Run("parcelas minimas", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{AmountCents: 100, Installments: 1})
		require.ErrorIs(t, err, intent.ErrInstallmentsTooFew)
	})

	t.Run("parcelas excessivas", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{AmountCents: 100, Installments: 25})
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

func TestNewCreateCard(t *testing.T) {
	t.Parallel()

	t.Run("valido com name e nickname", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewCreateCard(intent.CreateCardFields{
			Nickname:   "  Roxinho  ",
			Name:       "Itau Roxinho",
			ClosingDay: 10,
			DueDay:     17,
			LimitCents: 500000,
		})
		require.NoError(t, err)
		require.Equal(t, intent.KindCreateCard, got.Kind())
		require.Equal(t, "Roxinho", got.CardNickname())
		require.Equal(t, "Itau Roxinho", got.CardName())
		require.Equal(t, 10, got.ClosingDay())
		require.Equal(t, 17, got.DueDay())
		require.Equal(t, int64(500000), got.LimitCents())
	})

	t.Run("nickname vazio falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "   "})
		require.ErrorIs(t, err, intent.ErrCardNicknameEmpty)
	})

	t.Run("nao valida regra de dias", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "nubank", ClosingDay: 99, DueDay: 0})
		require.NoError(t, err)
		require.Equal(t, 99, got.ClosingDay())
		require.Equal(t, 0, got.DueDay())
	})

	t.Run("name muito longo falha", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("a", 121)
		_, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "nubank", Name: long})
		require.ErrorIs(t, err, intent.ErrCardNameTooLong)
	})
}

func TestNewCountCards(t *testing.T) {
	t.Parallel()
	got := intent.NewCountCards()
	require.Equal(t, intent.KindCountCards, got.Kind())
}

func TestPhase3bKinds_StringAndParseRoundTrip(t *testing.T) {
	t.Parallel()

	cases := map[intent.Kind]string{
		intent.KindUpdateCard:             "update_card",
		intent.KindDeleteCard:             "delete_card",
		intent.KindEditCategoryPercentage: "edit_category_percentage",
	}
	for k, want := range cases {
		require.Equal(t, want, k.String())
		parsed, err := intent.ParseKind(k.String())
		require.NoError(t, err)
		require.Equal(t, k, parsed)
		parsedUpper, err := intent.ParseKind(strings.ToUpper("  " + want + "  "))
		require.NoError(t, err)
		require.Equal(t, k, parsedUpper)
	}
}

func TestPhase3bKinds_IsWrite(t *testing.T) {
	t.Parallel()

	kinds := []intent.Kind{
		intent.KindUpdateCard,
		intent.KindDeleteCard,
		intent.KindEditCategoryPercentage,
	}
	for _, k := range kinds {
		require.True(t, k.IsWrite(), "kind %v deve ser escrita", k)
	}
}

func strptr(s string) *string { return &s }
func intptr(v int) *int       { return &v }

func TestNewUpdateCard(t *testing.T) {
	t.Parallel()

	t.Run("valido com nickname", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewUpdateCard(intent.UpdateCardFields{
			CardName: "  Roxinho  ",
			Nickname: strptr("  Itau Black  "),
		})
		require.NoError(t, err)
		require.Equal(t, intent.KindUpdateCard, got.Kind())
		require.Equal(t, "Roxinho", got.CardName())
		require.NotNil(t, got.NicknamePtr())
		require.Equal(t, "Itau Black", *got.NicknamePtr())
		require.Nil(t, got.NamePtr())
		require.Nil(t, got.ClosingDayPtr())
		require.Nil(t, got.DueDayPtr())
	})

	t.Run("valido com dias", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewUpdateCard(intent.UpdateCardFields{
			CardName:   "nubank",
			ClosingDay: intptr(10),
			DueDay:     intptr(17),
		})
		require.NoError(t, err)
		require.NotNil(t, got.ClosingDayPtr())
		require.Equal(t, 10, *got.ClosingDayPtr())
		require.NotNil(t, got.DueDayPtr())
		require.Equal(t, 17, *got.DueDayPtr())
	})

	t.Run("card name vazio falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "   ", Nickname: strptr("x")})
		require.ErrorIs(t, err, intent.ErrCardNameEmpty)
	})

	t.Run("card name muito longo falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: strings.Repeat("a", 121), Nickname: strptr("x")})
		require.ErrorIs(t, err, intent.ErrCardNameTooLong)
	})

	t.Run("sem campos para alterar falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank"})
		require.ErrorIs(t, err, intent.ErrNoFieldsToUpdate)
	})

	t.Run("closing day invalido falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", ClosingDay: intptr(0)})
		require.ErrorIs(t, err, intent.ErrCardDayInvalid)
	})

	t.Run("due day invalido falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", DueDay: intptr(32)})
		require.ErrorIs(t, err, intent.ErrCardDayInvalid)
	})

	t.Run("nickname muito longo falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", Nickname: strptr(strings.Repeat("a", 121))})
		require.ErrorIs(t, err, intent.ErrCardNicknameTooLong)
	})

	t.Run("name muito longo falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", Name: strptr(strings.Repeat("a", 121))})
		require.ErrorIs(t, err, intent.ErrCardNameTooLong)
	})
}

func TestNewDeleteCard(t *testing.T) {
	t.Parallel()

	t.Run("valido", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewDeleteCard("  Nubank  ")
		require.NoError(t, err)
		require.Equal(t, intent.KindDeleteCard, got.Kind())
		require.Equal(t, "Nubank", got.CardName())
	})

	t.Run("vazio falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewDeleteCard("   ")
		require.ErrorIs(t, err, intent.ErrCardNameEmpty)
	})

	t.Run("muito longo falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewDeleteCard(strings.Repeat("a", 121))
		require.ErrorIs(t, err, intent.ErrCardNameTooLong)
	})
}

func TestNewEditCategoryPercentage(t *testing.T) {
	t.Parallel()

	t.Run("valido", func(t *testing.T) {
		t.Parallel()
		got, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
			CategoryName: "  Lazer  ",
			Percentage:   30,
		})
		require.NoError(t, err)
		require.Equal(t, intent.KindEditCategoryPercentage, got.Kind())
		require.Equal(t, "Lazer", got.CategoryName())
		require.Equal(t, 30, got.Percentage())
	})

	t.Run("limites validos", func(t *testing.T) {
		t.Parallel()
		zero, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: "Casa", Percentage: 0})
		require.NoError(t, err)
		require.Equal(t, 0, zero.Percentage())

		full, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: "Casa", Percentage: 100})
		require.NoError(t, err)
		require.Equal(t, 100, full.Percentage())
	})

	t.Run("categoria vazia falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: "  ", Percentage: 10})
		require.ErrorIs(t, err, intent.ErrCategoryNameEmpty)
	})

	t.Run("categoria muito longa falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: strings.Repeat("a", 121), Percentage: 10})
		require.ErrorIs(t, err, intent.ErrCategoryNameTooLong)
	})

	t.Run("percentual negativo falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: "Casa", Percentage: -1})
		require.ErrorIs(t, err, intent.ErrPercentageOutOfRange)
	})

	t.Run("percentual acima de 100 falha", func(t *testing.T) {
		t.Parallel()
		_, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: "Casa", Percentage: 101})
		require.ErrorIs(t, err, intent.ErrPercentageOutOfRange)
	})
}
