package intent

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type KindRegistrySuite struct {
	suite.Suite
}

func TestKindRegistrySuite(t *testing.T) {
	suite.Run(t, new(KindRegistrySuite))
}

func allKinds() []Kind {
	kinds := make([]Kind, 0, int(KindBudgetRecurrence))
	for k := KindUnknown; k <= KindBudgetRecurrence; k++ {
		kinds = append(kinds, k)
	}
	return kinds
}

func kindBuilders() map[Kind]func() (Intent, error) {
	nickname := "nubank"
	name := "Nubank Roxinho"
	return map[Kind]func() (Intent, error){
		KindUnknown: func() (Intent, error) {
			return NewUnknown("texto qualquer")
		},
		KindRecordExpense: func() (Intent, error) {
			return NewRecordExpense(RecordExpenseFields{AmountCents: 100, PaymentMethod: "pix"})
		},
		KindQueryCategory: func() (Intent, error) {
			return NewQueryCategory("Lazer")
		},
		KindQueryGoal: func() (Intent, error) {
			return NewQueryGoal("Europa")
		},
		KindQueryCard: func() (Intent, error) {
			return NewQueryCard("nubank")
		},
		KindMonthlySummary: func() (Intent, error) {
			return NewMonthlySummary("")
		},
		KindHowAmIDoing: func() (Intent, error) {
			return NewHowAmIDoing(), nil
		},
		KindConfigureBudget: func() (Intent, error) {
			return NewConfigureBudget(ConfigureBudgetFields{})
		},
		KindRecordIncome: func() (Intent, error) {
			return NewRecordIncome(RecordIncomeFields{AmountCents: 100, PaymentMethod: "pix"})
		},
		KindRecordCardPurchase: func() (Intent, error) {
			return NewRecordCardPurchase(RecordCardPurchaseFields{AmountCents: 100, Installments: 2})
		},
		KindListTransactions: func() (Intent, error) {
			return NewListTransactions("")
		},
		KindDeleteLastTransaction: func() (Intent, error) {
			return NewDeleteLastTransaction(), nil
		},
		KindEditLastTransaction: func() (Intent, error) {
			return NewEditLastTransaction(100)
		},
		KindCreateRecurring: func() (Intent, error) {
			return NewCreateRecurring(CreateRecurringFields{AmountCents: 100, Direction: "income", DayOfMonth: 5})
		},
		KindListRecurring: func() (Intent, error) {
			return NewListRecurring(), nil
		},
		KindListCards: func() (Intent, error) {
			return NewListCards(), nil
		},
		KindCreateCard: func() (Intent, error) {
			return NewCreateCard(CreateCardFields{Nickname: nickname})
		},
		KindCountCards: func() (Intent, error) {
			return NewCountCards(), nil
		},
		KindUpdateCard: func() (Intent, error) {
			return NewUpdateCard(UpdateCardFields{CardName: "nubank", Name: &name})
		},
		KindDeleteCard: func() (Intent, error) {
			return NewDeleteCard("nubank")
		},
		KindEditCategoryPercentage: func() (Intent, error) {
			return NewEditCategoryPercentage(EditCategoryPercentageFields{CategoryName: "Lazer", Percentage: 50})
		},
		KindQueryIncomeSummary: func() (Intent, error) {
			return NewQueryIncomeSummary("")
		},
		KindBudgetRecurrence: func() (Intent, error) {
			return NewBudgetRecurrence(BudgetRecurrenceFields{SourceCompetence: "2026-06", Months: 3})
		},
	}
}

func (s *KindRegistrySuite) TestRoundTrip() {
	for _, k := range allKinds() {
		slug := k.String()
		got, err := ParseKind(slug)
		s.NoError(err, "ParseKind(%q) falhou para kind=%d", slug, k)
		s.Equal(k, got, "round-trip falhou para kind=%d slug=%q", k, slug)
	}
}

func (s *KindRegistrySuite) TestSlugNotDefaultForNonUnknown() {
	for _, k := range allKinds() {
		if k == KindUnknown {
			continue
		}
		slug := k.String()
		s.NotEqual("unknown", slug, "kind=%d caiu no default 'unknown'", k)
	}
}

func (s *KindRegistrySuite) TestAllKindsHaveBuilder() {
	builders := kindBuilders()
	for _, k := range allKinds() {
		builder, exists := builders[k]
		s.Require().True(exists, "kind %d (%q) não tem builder em kindBuilders()", k, k.String())
		got, err := builder()
		s.NoError(err, "builder de kind %d (%q) retornou erro", k, k.String())
		s.Equal(k, got.Kind(), "builder de kind %d produziu kind errado: got %d", k, got.Kind())
	}
}

func (s *KindRegistrySuite) TestBuildersMapExhaustiveness() {
	builders := kindBuilders()
	allK := allKinds()
	for _, k := range allK {
		_, exists := builders[k]
		s.True(exists, "kindBuilders() não cobre kind %d (%q) — mapa está incompleto", k, k.String())
	}
	for k := range builders {
		found := false
		for _, ak := range allK {
			if ak == k {
				found = true
				break
			}
		}
		s.True(found, "kindBuilders() tem entrada extra para kind %d que não existe em allKinds()", k)
	}
}
