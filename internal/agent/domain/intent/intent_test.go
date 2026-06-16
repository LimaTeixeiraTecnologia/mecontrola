package intent_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestKindString(t *testing.T) {
	t.Parallel()

	cases := map[intent.Kind]string{
		intent.KindLogExpense:      "log_expense",
		intent.KindQueryCategory:   "query_category",
		intent.KindQueryGoal:       "query_goal",
		intent.KindQueryCard:       "query_card",
		intent.KindMonthlySummary:  "monthly_summary",
		intent.KindHowAmIDoing:     "how_am_i_doing",
		intent.KindConfigureBudget: "configure_budget",
		intent.KindUnknown:         "unknown",
	}

	for k, want := range cases {
		if got := k.String(); got != want {
			t.Fatalf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestParseKind(t *testing.T) {
	t.Parallel()

	type tc struct {
		in      string
		want    intent.Kind
		wantErr error
	}

	tests := []tc{
		{in: "log_expense", want: intent.KindLogExpense},
		{in: " QUERY_CATEGORY ", want: intent.KindQueryCategory},
		{in: "query_goal", want: intent.KindQueryGoal},
		{in: "query_card", want: intent.KindQueryCard},
		{in: "monthly_summary", want: intent.KindMonthlySummary},
		{in: "how_am_i_doing", want: intent.KindHowAmIDoing},
		{in: "configure_budget", want: intent.KindConfigureBudget},
		{in: " CONFIGURE_BUDGET ", want: intent.KindConfigureBudget},
		{in: "unknown", want: intent.KindUnknown},
		{in: "", want: intent.KindUnknown},
		{in: "bogus", wantErr: intent.ErrKindUnknown},
	}

	for _, tc := range tests {
		got, err := intent.ParseKind(tc.in)
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseKind(%q) err = %v, want %v", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseKind(%q) unexpected err: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseKind(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewLogExpense(t *testing.T) {
	t.Parallel()

	type tc struct {
		name    string
		fields  intent.LogExpenseFields
		wantErr error
		check   func(t *testing.T, got intent.Intent)
	}

	tests := []tc{
		{
			name: "happy_path_credit",
			fields: intent.LogExpenseFields{
				AmountCents:   5800,
				Merchant:      "iFood",
				CategoryHint:  "Alimentação",
				PaymentMethod: "CREDIT",
				CardHint:      "nubank",
			},
			check: func(t *testing.T, got intent.Intent) {
				if got.Kind() != intent.KindLogExpense {
					t.Fatalf("kind = %v", got.Kind())
				}
				if got.AmountCents() != 5800 {
					t.Fatalf("amount = %d", got.AmountCents())
				}
				if got.PaymentMethod() != "credit" {
					t.Fatalf("payment_method = %q", got.PaymentMethod())
				}
				if got.Merchant() != "iFood" {
					t.Fatalf("merchant = %q", got.Merchant())
				}
			},
		},
		{
			name:    "negative_amount",
			fields:  intent.LogExpenseFields{AmountCents: -1},
			wantErr: intent.ErrAmountNonPositive,
		},
		{
			name:    "zero_amount",
			fields:  intent.LogExpenseFields{AmountCents: 0},
			wantErr: intent.ErrAmountNonPositive,
		},
		{
			name: "merchant_too_long",
			fields: intent.LogExpenseFields{
				AmountCents: 100,
				Merchant:    strings.Repeat("a", 200),
			},
			wantErr: intent.ErrMerchantTooLong,
		},
		{
			name: "invalid_payment_method",
			fields: intent.LogExpenseFields{
				AmountCents:   100,
				PaymentMethod: "bitcoin",
			},
			wantErr: intent.ErrPaymentMethodInvalid,
		},
		{
			name: "empty_payment_method_ok",
			fields: intent.LogExpenseFields{
				AmountCents: 100,
			},
			check: func(t *testing.T, got intent.Intent) {
				if got.PaymentMethod() != "" {
					t.Fatalf("payment_method = %q", got.PaymentMethod())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := intent.NewLogExpense(tc.fields)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestNewQueryCategory(t *testing.T) {
	t.Parallel()

	if _, err := intent.NewQueryCategory(""); !errors.Is(err, intent.ErrCategoryNameEmpty) {
		t.Fatalf("empty: err = %v", err)
	}
	got, err := intent.NewQueryCategory("  Prazeres  ")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Kind() != intent.KindQueryCategory || got.CategoryName() != "Prazeres" {
		t.Fatalf("got = %+v", got)
	}
}

func TestNewQueryGoal(t *testing.T) {
	t.Parallel()

	if _, err := intent.NewQueryGoal("   "); !errors.Is(err, intent.ErrGoalNameEmpty) {
		t.Fatalf("empty: err = %v", err)
	}
	got, err := intent.NewQueryGoal("Viagem Europa")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Kind() != intent.KindQueryGoal || got.GoalName() != "Viagem Europa" {
		t.Fatalf("got = %+v", got)
	}
}

func TestNewQueryCard(t *testing.T) {
	t.Parallel()

	if _, err := intent.NewQueryCard(""); !errors.Is(err, intent.ErrCardNameEmpty) {
		t.Fatalf("empty: err = %v", err)
	}
	got, err := intent.NewQueryCard("Nubank")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Kind() != intent.KindQueryCard || got.CardName() != "Nubank" {
		t.Fatalf("got = %+v", got)
	}
}

func TestNewMonthlySummary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		wantRef string
		wantErr error
	}{
		{in: "", wantRef: ""},
		{in: "  ", wantRef: ""},
		{in: "2026-06", wantRef: "2026-06"},
		{in: "2026-13", wantErr: intent.ErrRefMonthInvalid},
		{in: "2026/06", wantErr: intent.ErrRefMonthInvalid},
		{in: "26-06", wantErr: intent.ErrRefMonthInvalid},
	}
	for _, tc := range cases {
		got, err := intent.NewMonthlySummary(tc.in)
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("in=%q err=%v want=%v", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("in=%q unexpected: %v", tc.in, err)
		}
		if got.Kind() != intent.KindMonthlySummary {
			t.Fatalf("in=%q kind=%v", tc.in, got.Kind())
		}
		if got.RefMonth() != tc.wantRef {
			t.Fatalf("in=%q ref=%q want=%q", tc.in, got.RefMonth(), tc.wantRef)
		}
	}
}

func TestNewHowAmIDoing(t *testing.T) {
	t.Parallel()
	got := intent.NewHowAmIDoing()
	if got.Kind() != intent.KindHowAmIDoing {
		t.Fatalf("kind = %v", got.Kind())
	}
}

func TestNewConfigureBudget(t *testing.T) {
	t.Parallel()
	got := intent.NewConfigureBudget()
	if got.Kind() != intent.KindConfigureBudget {
		t.Fatalf("kind = %v", got.Kind())
	}
	if got.IsZero() {
		t.Fatalf("intent should not be zero")
	}
}

func TestNewUnknown(t *testing.T) {
	t.Parallel()

	if _, err := intent.NewUnknown(""); !errors.Is(err, intent.ErrRawTextEmpty) {
		t.Fatalf("empty: err = %v", err)
	}
	got, err := intent.NewUnknown("  oi tudo bem  ")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Kind() != intent.KindUnknown || got.RawText() != "oi tudo bem" {
		t.Fatalf("got = %+v", got)
	}
}
