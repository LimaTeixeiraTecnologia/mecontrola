package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type fakeInterpreter struct {
	resp        interfaces.LLMResponse
	err         error
	lastRequest interfaces.LLMRequest
}

func (f *fakeInterpreter) Interpret(_ context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	f.lastRequest = req
	return f.resp, f.err
}

func newSUT(t *testing.T, resp string, err error) *usecases.ParseInbound {
	t.Helper()
	uc, ucErr := usecases.NewParseInbound(&fakeInterpreter{
		resp: interfaces.LLMResponse{RawJSON: []byte(resp)},
		err:  err,
	}, noop.NewProvider())
	if ucErr != nil {
		t.Fatalf("NewParseInbound: %v", ucErr)
	}
	return uc
}

func TestParseInbound_NewParseInbound_NilDeps(t *testing.T) {
	t.Parallel()
	if _, err := usecases.NewParseInbound(nil, noop.NewProvider()); err == nil {
		t.Fatalf("expected error for nil interpreter")
	}
	if _, err := usecases.NewParseInbound(&fakeInterpreter{}, nil); err == nil {
		t.Fatalf("expected error for nil o11y")
	}
}

func TestParseInbound_Execute_EmptyText(t *testing.T) {
	t.Parallel()
	uc := newSUT(t, `{"kind":"unknown","raw_text":"x"}`, nil)
	_, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "   ",
	})
	if !errors.Is(err, usecases.ErrParseInboundEmptyText) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseInbound_Execute_AllKinds(t *testing.T) { //nolint:revive // tabela exaustiva por intent kind
	t.Parallel()

	cases := []struct {
		name    string
		llmJSON string
		want    intent.Kind
		check   func(t *testing.T, got intent.Intent)
	}{
		{
			name:    "log_expense",
			llmJSON: `{"kind":"log_expense","amount_cents":5800,"merchant":"iFood","category_hint":"Alimentação","payment_method":"credit","card_hint":"nubank"}`,
			want:    intent.KindLogExpense,
			check: func(t *testing.T, got intent.Intent) {
				if got.AmountCents() != 5800 || got.Merchant() != "iFood" || got.PaymentMethod() != "credit" {
					t.Fatalf("got = %+v", got)
				}
			},
		},
		{
			name:    "query_category",
			llmJSON: `{"kind":"query_category","category_name":"Prazeres"}`,
			want:    intent.KindQueryCategory,
			check: func(t *testing.T, got intent.Intent) {
				if got.CategoryName() != "Prazeres" {
					t.Fatalf("category_name = %q", got.CategoryName())
				}
			},
		},
		{
			name:    "query_goal",
			llmJSON: `{"kind":"query_goal","goal_name":"Viagem"}`,
			want:    intent.KindQueryGoal,
		},
		{
			name:    "query_card",
			llmJSON: `{"kind":"query_card","card_name":"nubank"}`,
			want:    intent.KindQueryCard,
		},
		{
			name:    "monthly_summary_with_ref_month",
			llmJSON: `{"kind":"monthly_summary","ref_month":"2026-06"}`,
			want:    intent.KindMonthlySummary,
			check: func(t *testing.T, got intent.Intent) {
				if got.RefMonth() != "2026-06" {
					t.Fatalf("ref_month = %q", got.RefMonth())
				}
			},
		},
		{
			name:    "monthly_summary_no_ref_month",
			llmJSON: `{"kind":"monthly_summary"}`,
			want:    intent.KindMonthlySummary,
		},
		{
			name:    "how_am_i_doing",
			llmJSON: `{"kind":"how_am_i_doing"}`,
			want:    intent.KindHowAmIDoing,
		},
		{
			name:    "unknown_with_raw_text",
			llmJSON: `{"kind":"unknown","raw_text":"oi bom dia"}`,
			want:    intent.KindUnknown,
			check: func(t *testing.T, got intent.Intent) {
				if got.RawText() != "oi bom dia" {
					t.Fatalf("raw_text = %q", got.RawText())
				}
			},
		},
		{
			name:    "fenced_json",
			llmJSON: "```json\n{\"kind\":\"how_am_i_doing\"}\n```",
			want:    intent.KindHowAmIDoing,
		},
		{
			name:    "log_card_purchase",
			llmJSON: `{"kind":"log_card_purchase","amount_cents":120000,"merchant":"supermercado","card_hint":"nubank","installments":6}`,
			want:    intent.KindLogCardPurchase,
			check: func(t *testing.T, got intent.Intent) {
				if got.AmountCents() != 120000 || got.Installments() != 6 || got.CardHint() != "nubank" {
					t.Fatalf("got = %+v", got)
				}
			},
		},
		{
			name:    "list_transactions",
			llmJSON: `{"kind":"list_transactions","ref_month":"2026-06"}`,
			want:    intent.KindListTransactions,
			check: func(t *testing.T, got intent.Intent) {
				if got.RefMonth() != "2026-06" {
					t.Fatalf("ref_month = %q", got.RefMonth())
				}
			},
		},
		{
			name:    "delete_last_transaction",
			llmJSON: `{"kind":"delete_last_transaction"}`,
			want:    intent.KindDeleteLastTransaction,
		},
		{
			name:    "edit_last_transaction",
			llmJSON: `{"kind":"edit_last_transaction","amount_cents":8000}`,
			want:    intent.KindEditLastTransaction,
			check: func(t *testing.T, got intent.Intent) {
				if got.AmountCents() != 8000 {
					t.Fatalf("amount_cents = %d", got.AmountCents())
				}
			},
		},
		{
			name:    "create_recurring_explicit_direction",
			llmJSON: `{"kind":"create_recurring","amount_cents":500000,"merchant":"salário","direction":"income","frequency":"monthly","day_of_month":5}`,
			want:    intent.KindCreateRecurring,
			check: func(t *testing.T, got intent.Intent) {
				if got.Direction() != "income" || got.Frequency() != "monthly" || got.DayOfMonth() != 5 {
					t.Fatalf("got = %+v", got)
				}
			},
		},
		{
			name:    "create_recurring_infers_outcome_default",
			llmJSON: `{"kind":"create_recurring","amount_cents":120000,"merchant":"aluguel","day_of_month":0}`,
			want:    intent.KindCreateRecurring,
			check: func(t *testing.T, got intent.Intent) {
				if got.Direction() != "outcome" || got.Frequency() != "monthly" || got.DayOfMonth() != 1 {
					t.Fatalf("got = %+v", got)
				}
			},
		},
		{
			name:    "list_recurring",
			llmJSON: `{"kind":"list_recurring"}`,
			want:    intent.KindListRecurring,
		},
		{
			name:    "list_cards",
			llmJSON: `{"kind":"list_cards"}`,
			want:    intent.KindListCards,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := newSUT(t, tc.llmJSON, nil)
			out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
				UserID: uuid.New(),
				Text:   "qualquer texto",
			})
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if out.Intent.Kind() != tc.want {
				t.Fatalf("kind = %v, want %v", out.Intent.Kind(), tc.want)
			}
			if tc.check != nil {
				tc.check(t, out.Intent)
			}
		})
	}
}

func TestParseInbound_Execute_CreateRecurring_InfersIncomeFromText(t *testing.T) {
	t.Parallel()

	uc := newSUT(t, `{"kind":"create_recurring","amount_cents":500000,"merchant":"salário"}`, nil)
	out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "todo mês recebo 5000 de salário",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Intent.Kind() != intent.KindCreateRecurring {
		t.Fatalf("kind = %v", out.Intent.Kind())
	}
	if out.Intent.Direction() != "income" {
		t.Fatalf("direction = %q, want income (inferido do texto)", out.Intent.Direction())
	}
}

func TestParseInbound_Execute_InvalidJSON_Fallback(t *testing.T) {
	t.Parallel()

	uc := newSUT(t, `not a json`, nil)
	out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "preciso pagar a fatura",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Intent.Kind() != intent.KindUnknown {
		t.Fatalf("kind = %v", out.Intent.Kind())
	}
	if out.Intent.RawText() != "preciso pagar a fatura" {
		t.Fatalf("raw_text = %q", out.Intent.RawText())
	}
}

func TestParseInbound_Execute_MissingKind_Fallback(t *testing.T) {
	t.Parallel()

	uc := newSUT(t, `{"amount_cents":100}`, nil)
	out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "gastei algo",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Intent.Kind() != intent.KindUnknown {
		t.Fatalf("kind = %v", out.Intent.Kind())
	}
}

func TestParseInbound_Execute_DomainInvariantViolation_Fallback(t *testing.T) {
	t.Parallel()

	uc := newSUT(t, `{"kind":"log_expense","amount_cents":0}`, nil)
	out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "gastei algo",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Intent.Kind() != intent.KindUnknown {
		t.Fatalf("kind = %v", out.Intent.Kind())
	}
}

func TestParseInbound_Execute_ProviderError_Fallback(t *testing.T) {
	t.Parallel()

	uc := newSUT(t, ``, errors.New("upstream timeout"))
	out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "como tá meu cartão?",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Intent.Kind() != intent.KindUnknown {
		t.Fatalf("kind = %v", out.Intent.Kind())
	}
	if out.Intent.RawText() != "como tá meu cartão?" {
		t.Fatalf("raw_text = %q", out.Intent.RawText())
	}
}

func TestParseInbound_Execute_ForwardsJSONSchemaToInterpreter(t *testing.T) {
	t.Parallel()

	fake := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"how_am_i_doing"}`)}}
	uc, err := usecases.NewParseInbound(fake, noop.NewProvider())
	if err != nil {
		t.Fatalf("NewParseInbound: %v", err)
	}
	if _, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "como tá meu mês?",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fake.lastRequest.JSONSchema == nil {
		t.Fatalf("expected JSONSchema to be set on the LLMRequest")
	}
	if fake.lastRequest.JSONSchema.Name != "mecontrola_parse_intent" {
		t.Fatalf("schema name = %q", fake.lastRequest.JSONSchema.Name)
	}
	if !fake.lastRequest.JSONSchema.Strict {
		t.Fatalf("schema strict = false, want true")
	}
	props, ok := fake.lastRequest.JSONSchema.Schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing")
	}
	if _, ok := props["amount_cents"]; !ok {
		t.Fatalf("schema must include amount_cents (new parse_inbound schema)")
	}
	if _, ok := props["module"]; ok {
		t.Fatalf("schema must NOT include legacy 'module' field")
	}
}

func TestParseInbound_Execute_UnknownKindString_Fallback(t *testing.T) {
	t.Parallel()

	uc := newSUT(t, `{"kind":"bogus"}`, nil)
	out, err := uc.Execute(context.Background(), usecases.ParseInboundInput{
		UserID: uuid.New(),
		Text:   "texto",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Intent.Kind() != intent.KindUnknown {
		t.Fatalf("kind = %v", out.Intent.Kind())
	}
}
