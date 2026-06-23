package intent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type IntentSuite struct {
	suite.Suite
	ctx context.Context
}

func TestIntentSuite(t *testing.T) {
	suite.Run(t, new(IntentSuite))
}

func (s *IntentSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *IntentSuite) TestKindString() {
	cases := map[Kind]string{
		KindRecordExpense:   "record_expense",
		KindQueryCategory:   "query_category",
		KindQueryGoal:       "query_goal",
		KindQueryCard:       "query_card",
		KindMonthlySummary:  "monthly_summary",
		KindHowAmIDoing:     "how_am_i_doing",
		KindConfigureBudget: "configure_budget",
		KindListCards:       "list_cards",
		KindUnknown:         "unknown",
	}

	for k, want := range cases {
		got := k.String()
		s.Equal(want, got, "Kind(%d).String()", k)
	}
}

func (s *IntentSuite) TestParseKind() {
	type tc struct {
		in      string
		want    Kind
		wantErr error
	}

	tests := []tc{
		{in: "record_expense", want: KindRecordExpense},
		{in: " QUERY_CATEGORY ", want: KindQueryCategory},
		{in: "query_goal", want: KindQueryGoal},
		{in: "query_card", want: KindQueryCard},
		{in: "monthly_summary", want: KindMonthlySummary},
		{in: "how_am_i_doing", want: KindHowAmIDoing},
		{in: "configure_budget", want: KindConfigureBudget},
		{in: " CONFIGURE_BUDGET ", want: KindConfigureBudget},
		{in: "list_cards", want: KindListCards},
		{in: "unknown", want: KindUnknown},
		{in: "", want: KindUnknown},
		{in: "bogus", wantErr: ErrKindUnknown},
	}

	for _, tc := range tests {
		got, err := ParseKind(tc.in)
		if tc.wantErr != nil {
			s.True(errors.Is(err, tc.wantErr), "ParseKind(%q) err = %v, want %v", tc.in, err, tc.wantErr)
			continue
		}
		s.NoError(err, "ParseKind(%q) unexpected err", tc.in)
		s.Equal(tc.want, got, "ParseKind(%q)", tc.in)
	}
}

func (s *IntentSuite) TestNewRecordExpense() {
	type tc struct {
		name    string
		fields  RecordExpenseFields
		wantErr error
		check   func(got Intent)
	}

	tests := []tc{
		{
			name: "happy_path_credit",
			fields: RecordExpenseFields{
				AmountCents:   5800,
				Merchant:      "iFood",
				CategoryHint:  "Alimentação",
				PaymentMethod: "CREDIT",
				CardHint:      "nubank",
			},
			check: func(got Intent) {
				s.Equal(KindRecordExpense, got.Kind())
				s.Equal(int64(5800), got.AmountCents())
				s.Equal("credit", got.PaymentMethod())
				s.Equal("iFood", got.Merchant())
			},
		},
		{
			name:    "negative_amount",
			fields:  RecordExpenseFields{AmountCents: -1},
			wantErr: ErrAmountNonPositive,
		},
		{
			name:    "zero_amount",
			fields:  RecordExpenseFields{AmountCents: 0},
			wantErr: ErrAmountNonPositive,
		},
		{
			name: "merchant_too_long",
			fields: RecordExpenseFields{
				AmountCents: 100,
				Merchant:    strings.Repeat("a", 200),
			},
			wantErr: ErrMerchantTooLong,
		},
		{
			name: "invalid_payment_method",
			fields: RecordExpenseFields{
				AmountCents:   100,
				PaymentMethod: "bitcoin",
			},
			wantErr: ErrPaymentMethodInvalid,
		},
		{
			name: "empty_payment_method_ok",
			fields: RecordExpenseFields{
				AmountCents: 100,
			},
			check: func(got Intent) {
				s.Equal("", got.PaymentMethod())
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			got, err := NewRecordExpense(tc.fields)
			if tc.wantErr != nil {
				s.True(errors.Is(err, tc.wantErr), "err = %v, want %v", err, tc.wantErr)
				return
			}
			s.NoError(err)
			if tc.check != nil {
				tc.check(got)
			}
		})
	}
}

func (s *IntentSuite) TestNewQueryCategory() {
	_, err := NewQueryCategory("")
	s.True(errors.Is(err, ErrCategoryNameEmpty), "empty: err = %v", err)

	got, err := NewQueryCategory("  Prazeres  ")
	s.NoError(err)
	s.Equal(KindQueryCategory, got.Kind())
	s.Equal("Prazeres", got.CategoryName())
}

func (s *IntentSuite) TestNewQueryGoal() {
	empty, err := NewQueryGoal("   ")
	s.NoError(err)
	s.Equal(KindQueryGoal, empty.Kind())
	s.Equal("", empty.GoalName())

	got, err := NewQueryGoal("Viagem Europa")
	s.NoError(err)
	s.Equal(KindQueryGoal, got.Kind())
	s.Equal("Viagem Europa", got.GoalName())
}

func (s *IntentSuite) TestNewQueryCard() {
	empty, err := NewQueryCard("")
	s.NoError(err)
	s.Equal(KindQueryCard, empty.Kind())
	s.Equal("", empty.CardName())

	got, err := NewQueryCard("Nubank")
	s.NoError(err)
	s.Equal(KindQueryCard, got.Kind())
	s.Equal("Nubank", got.CardName())
}

func (s *IntentSuite) TestNewMonthlySummary() {
	cases := []struct {
		in      string
		wantRef string
		wantErr error
	}{
		{in: "", wantRef: ""},
		{in: "  ", wantRef: ""},
		{in: "2026-06", wantRef: "2026-06"},
		{in: "2026-13", wantErr: ErrRefMonthInvalid},
		{in: "2026/06", wantErr: ErrRefMonthInvalid},
		{in: "26-06", wantErr: ErrRefMonthInvalid},
	}
	for _, tc := range cases {
		got, err := NewMonthlySummary(tc.in)
		if tc.wantErr != nil {
			s.True(errors.Is(err, tc.wantErr), "in=%q err=%v want=%v", tc.in, err, tc.wantErr)
			continue
		}
		s.NoError(err, "in=%q unexpected", tc.in)
		s.Equal(KindMonthlySummary, got.Kind(), "in=%q kind", tc.in)
		s.Equal(tc.wantRef, got.RefMonth(), "in=%q ref", tc.in)
	}
}

func (s *IntentSuite) TestNewHowAmIDoing() {
	got := NewHowAmIDoing()
	s.Equal(KindHowAmIDoing, got.Kind())
}

func (s *IntentSuite) TestNewConfigureBudget() {
	got := NewConfigureBudget()
	s.Equal(KindConfigureBudget, got.Kind())
	s.False(got.IsZero())
}

func (s *IntentSuite) TestNewListCards() {
	got := NewListCards()
	s.Equal(KindListCards, got.Kind())
}

func (s *IntentSuite) TestNewUnknown() {
	_, err := NewUnknown("")
	s.True(errors.Is(err, ErrRawTextEmpty), "empty: err = %v", err)

	got, err := NewUnknown("  oi tudo bem  ")
	s.NoError(err)
	s.Equal(KindUnknown, got.Kind())
	s.Equal("oi tudo bem", got.RawText())
}
