package intent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type IntentNewKindsSuite struct {
	suite.Suite
	ctx context.Context
}

func TestIntentNewKindsSuite(t *testing.T) {
	suite.Run(t, new(IntentNewKindsSuite))
}

func (s *IntentNewKindsSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *IntentNewKindsSuite) TestNewKinds_StringAndParseRoundTrip() {
	kinds := []Kind{
		KindRecordCardPurchase,
		KindListTransactions,
		KindDeleteLastTransaction,
		KindEditLastTransaction,
		KindCreateRecurring,
		KindListRecurring,
		KindCreateCard,
		KindCountCards,
	}
	for _, k := range kinds {
		parsed, err := ParseKind(k.String())
		s.NoError(err)
		s.Equal(k, parsed)
	}
}

func (s *IntentNewKindsSuite) TestNewRecordCardPurchase() {
	s.Run("valido", func() {
		got, err := NewRecordCardPurchase(RecordCardPurchaseFields{
			AmountCents:  125090,
			Merchant:     "Magalu",
			CategoryHint: "Casa",
			CardHint:     "nubank",
			Installments: 12,
		})
		s.NoError(err)
		s.Equal(KindRecordCardPurchase, got.Kind())
		s.Equal(int64(125090), got.AmountCents())
		s.Equal(12, got.Installments())
		s.Equal("nubank", got.CardHint())
	})

	s.Run("amount nao positivo", func() {
		_, err := NewRecordCardPurchase(RecordCardPurchaseFields{AmountCents: 0, Installments: 3})
		s.ErrorIs(err, ErrAmountNonPositive)
	})

	s.Run("parcelas minimas", func() {
		_, err := NewRecordCardPurchase(RecordCardPurchaseFields{AmountCents: 100, Installments: 1})
		s.ErrorIs(err, ErrInstallmentsTooFew)
	})

	s.Run("parcelas excessivas", func() {
		_, err := NewRecordCardPurchase(RecordCardPurchaseFields{AmountCents: 100, Installments: 25})
		s.ErrorIs(err, ErrInstallmentsTooMany)
	})
}

func (s *IntentNewKindsSuite) TestNewListTransactions() {
	empty, err := NewListTransactions("")
	s.NoError(err)
	s.Equal(KindListTransactions, empty.Kind())
	s.Empty(empty.RefMonth())

	withMonth, err := NewListTransactions("2026-05")
	s.NoError(err)
	s.Equal("2026-05", withMonth.RefMonth())

	_, err = NewListTransactions("2026/05")
	s.ErrorIs(err, ErrRefMonthInvalid)
}

func (s *IntentNewKindsSuite) TestNewDeleteLastTransaction() {
	got := NewDeleteLastTransaction()
	s.Equal(KindDeleteLastTransaction, got.Kind())
}

func (s *IntentNewKindsSuite) TestNewEditLastTransaction() {
	got, err := NewEditLastTransaction(8000)
	s.NoError(err)
	s.Equal(KindEditLastTransaction, got.Kind())
	s.Equal(int64(8000), got.AmountCents())

	_, err = NewEditLastTransaction(0)
	s.ErrorIs(err, ErrAmountNonPositive)
}

func (s *IntentNewKindsSuite) TestNewCreateRecurring() {
	s.Run("default frequency monthly", func() {
		got, err := NewCreateRecurring(CreateRecurringFields{
			AmountCents: 500000,
			Merchant:    "salário",
			Direction:   "income",
			DayOfMonth:  5,
		})
		s.NoError(err)
		s.Equal(KindCreateRecurring, got.Kind())
		s.Equal("income", got.Direction())
		s.Equal("monthly", got.Frequency())
		s.Equal(5, got.DayOfMonth())
	})

	s.Run("direction invalido", func() {
		_, err := NewCreateRecurring(CreateRecurringFields{AmountCents: 100, Direction: "x", DayOfMonth: 1})
		s.ErrorIs(err, ErrDirectionInvalid)
	})

	s.Run("frequency invalida", func() {
		_, err := NewCreateRecurring(CreateRecurringFields{AmountCents: 100, Direction: "outcome", Frequency: "weekly", DayOfMonth: 1})
		s.ErrorIs(err, ErrFrequencyInvalid)
	})

	s.Run("day_of_month invalido", func() {
		_, err := NewCreateRecurring(CreateRecurringFields{AmountCents: 100, Direction: "outcome", DayOfMonth: 32})
		s.ErrorIs(err, ErrDayOfMonthInvalid)
	})

	s.Run("amount invalido", func() {
		_, err := NewCreateRecurring(CreateRecurringFields{AmountCents: 0, Direction: "income", DayOfMonth: 1})
		s.ErrorIs(err, ErrAmountNonPositive)
	})
}

func (s *IntentNewKindsSuite) TestNewListRecurring() {
	got := NewListRecurring()
	s.Equal(KindListRecurring, got.Kind())
}

func (s *IntentNewKindsSuite) TestNewCreateCard() {
	s.Run("valido com name e nickname", func() {
		got, err := NewCreateCard(CreateCardFields{
			Nickname:   "  Roxinho  ",
			Name:       "Itau Roxinho",
			ClosingDay: 10,
			DueDay:     17,
			LimitCents: 500000,
		})
		s.NoError(err)
		s.Equal(KindCreateCard, got.Kind())
		s.Equal("Roxinho", got.CardNickname())
		s.Equal("Itau Roxinho", got.CardName())
		s.Equal(10, got.ClosingDay())
		s.Equal(17, got.DueDay())
		s.Equal(int64(500000), got.LimitCents())
	})

	s.Run("nickname vazio falha", func() {
		_, err := NewCreateCard(CreateCardFields{Nickname: "   "})
		s.ErrorIs(err, ErrCardNicknameEmpty)
	})

	s.Run("nao valida regra de dias", func() {
		got, err := NewCreateCard(CreateCardFields{Nickname: "nubank", ClosingDay: 99, DueDay: 0})
		s.NoError(err)
		s.Equal(99, got.ClosingDay())
		s.Equal(0, got.DueDay())
	})

	s.Run("name muito longo falha", func() {
		long := strings.Repeat("a", 121)
		_, err := NewCreateCard(CreateCardFields{Nickname: "nubank", Name: long})
		s.ErrorIs(err, ErrCardNameTooLong)
	})
}

func (s *IntentNewKindsSuite) TestNewCountCards() {
	got := NewCountCards()
	s.Equal(KindCountCards, got.Kind())
}

func (s *IntentNewKindsSuite) TestPhase3bKinds_StringAndParseRoundTrip() {
	cases := map[Kind]string{
		KindUpdateCard:             "update_card",
		KindDeleteCard:             "delete_card",
		KindEditCategoryPercentage: "edit_category_percentage",
	}
	for k, want := range cases {
		s.Equal(want, k.String())
		parsed, err := ParseKind(k.String())
		s.NoError(err)
		s.Equal(k, parsed)
		parsedUpper, err := ParseKind(strings.ToUpper("  " + want + "  "))
		s.NoError(err)
		s.Equal(k, parsedUpper)
	}
}

func (s *IntentNewKindsSuite) TestPhase3bKinds_IsWrite() {
	kinds := []Kind{
		KindUpdateCard,
		KindDeleteCard,
		KindEditCategoryPercentage,
	}
	for _, k := range kinds {
		s.True(k.IsWrite(), "kind %v deve ser escrita", k)
	}
}

func (s *IntentNewKindsSuite) TestNewUpdateCard() {
	s.Run("valido com nickname", func() {
		nickname := "  Itau Black  "
		got, err := NewUpdateCard(UpdateCardFields{
			CardName: "  Roxinho  ",
			Nickname: &nickname,
		})
		s.NoError(err)
		s.Equal(KindUpdateCard, got.Kind())
		s.Equal("Roxinho", got.CardName())
		s.NotNil(got.NicknamePtr())
		s.Equal("Itau Black", *got.NicknamePtr())
		s.Nil(got.NamePtr())
		s.Nil(got.ClosingDayPtr())
		s.Nil(got.DueDayPtr())
	})

	s.Run("valido com dias", func() {
		closingDay := 10
		dueDay := 17
		got, err := NewUpdateCard(UpdateCardFields{
			CardName:   "nubank",
			ClosingDay: &closingDay,
			DueDay:     &dueDay,
		})
		s.NoError(err)
		s.NotNil(got.ClosingDayPtr())
		s.Equal(10, *got.ClosingDayPtr())
		s.NotNil(got.DueDayPtr())
		s.Equal(17, *got.DueDayPtr())
	})

	s.Run("card name vazio falha", func() {
		nx := "x"
		_, err := NewUpdateCard(UpdateCardFields{CardName: "   ", Nickname: &nx})
		s.ErrorIs(err, ErrCardNameEmpty)
	})

	s.Run("card name muito longo falha", func() {
		nx2 := "x"
		_, err := NewUpdateCard(UpdateCardFields{CardName: strings.Repeat("a", 121), Nickname: &nx2})
		s.ErrorIs(err, ErrCardNameTooLong)
	})

	s.Run("sem campos para alterar falha", func() {
		_, err := NewUpdateCard(UpdateCardFields{CardName: "nubank"})
		s.ErrorIs(err, ErrNoFieldsToUpdate)
	})

	s.Run("closing day invalido falha", func() {
		cd0 := 0
		_, err := NewUpdateCard(UpdateCardFields{CardName: "nubank", ClosingDay: &cd0})
		s.ErrorIs(err, ErrCardDayInvalid)
	})

	s.Run("due day invalido falha", func() {
		dd32 := 32
		_, err := NewUpdateCard(UpdateCardFields{CardName: "nubank", DueDay: &dd32})
		s.ErrorIs(err, ErrCardDayInvalid)
	})

	s.Run("nickname muito longo falha", func() {
		longNickname := strings.Repeat("a", 121)
		_, err := NewUpdateCard(UpdateCardFields{CardName: "nubank", Nickname: &longNickname})
		s.ErrorIs(err, ErrCardNicknameTooLong)
	})

	s.Run("name muito longo falha", func() {
		longName := strings.Repeat("a", 121)
		_, err := NewUpdateCard(UpdateCardFields{CardName: "nubank", Name: &longName})
		s.ErrorIs(err, ErrCardNameTooLong)
	})
}

func (s *IntentNewKindsSuite) TestNewDeleteCard() {
	s.Run("valido", func() {
		got, err := NewDeleteCard("  Nubank  ")
		s.NoError(err)
		s.Equal(KindDeleteCard, got.Kind())
		s.Equal("Nubank", got.CardName())
	})

	s.Run("vazio falha", func() {
		_, err := NewDeleteCard("   ")
		s.ErrorIs(err, ErrCardNameEmpty)
	})

	s.Run("muito longo falha", func() {
		_, err := NewDeleteCard(strings.Repeat("a", 121))
		s.ErrorIs(err, ErrCardNameTooLong)
	})
}

func (s *IntentNewKindsSuite) TestNewEditCategoryPercentage() {
	s.Run("valido", func() {
		got, err := NewEditCategoryPercentage(EditCategoryPercentageFields{
			CategoryName: "  Lazer  ",
			Percentage:   30,
		})
		s.NoError(err)
		s.Equal(KindEditCategoryPercentage, got.Kind())
		s.Equal("Lazer", got.CategoryName())
		s.Equal(30, got.Percentage())
	})

	s.Run("limites validos", func() {
		zero, err := NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: "Casa", Percentage: 0})
		s.NoError(err)
		s.Equal(0, zero.Percentage())

		full, err := NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: "Casa", Percentage: 100})
		s.NoError(err)
		s.Equal(100, full.Percentage())
	})

	s.Run("categoria vazia falha", func() {
		_, err := NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: "  ", Percentage: 10})
		s.ErrorIs(err, ErrCategoryNameEmpty)
	})

	s.Run("categoria muito longa falha", func() {
		_, err := NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: strings.Repeat("a", 121), Percentage: 10})
		s.ErrorIs(err, ErrCategoryNameTooLong)
	})

	s.Run("percentual negativo falha", func() {
		_, err := NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: "Casa", Percentage: -1})
		s.ErrorIs(err, ErrPercentageOutOfRange)
	})

	s.Run("percentual acima de 100 falha", func() {
		_, err := NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: "Casa", Percentage: 101})
		s.ErrorIs(err, ErrPercentageOutOfRange)
	})
}
