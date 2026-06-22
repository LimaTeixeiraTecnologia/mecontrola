package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
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

type ParseInboundSuite struct {
	suite.Suite
	ctx context.Context
}

func TestParseInboundSuite(t *testing.T) {
	suite.Run(t, new(ParseInboundSuite))
}

func (s *ParseInboundSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ParseInboundSuite) newSUT(resp string, err error) *ParseInbound {
	uc, ucErr := NewParseInbound(&fakeInterpreter{
		resp: interfaces.LLMResponse{RawJSON: []byte(resp)},
		err:  err,
	}, 2000, fake.NewProvider())
	s.Require().NoError(ucErr)
	return uc
}

func (s *ParseInboundSuite) TestNewParseInboundNilDeps() {
	_, err := NewParseInbound(nil, 2000, fake.NewProvider())
	s.Require().Error(err)

	_, err = NewParseInbound(&fakeInterpreter{}, 2000, nil)
	s.Require().Error(err)
}

func (s *ParseInboundSuite) TestExecuteEmptyText() {
	uc := s.newSUT(`{"kind":"unknown","raw_text":"x"}`, nil)
	_, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "   ",
	})
	s.Require().ErrorIs(err, ErrParseInboundEmptyText)
}

func (s *ParseInboundSuite) TestExecuteAllKinds() { //nolint:revive // tabela exaustiva por intent kind
	cases := []struct {
		name    string
		llmJSON string
		want    intent.Kind
		check   func(got intent.Intent)
	}{
		{
			name:    "log_expense",
			llmJSON: `{"kind":"log_expense","amount_cents":5800,"merchant":"iFood","category_hint":"Alimentação","payment_method":"credit","card_hint":"nubank"}`,
			want:    intent.KindLogExpense,
			check: func(got intent.Intent) {
				s.Equal(int64(5800), got.AmountCents())
				s.Equal("iFood", got.Merchant())
				s.Equal("credit", got.PaymentMethod())
			},
		},
		{
			name:    "query_category",
			llmJSON: `{"kind":"query_category","category_name":"Prazeres"}`,
			want:    intent.KindQueryCategory,
			check: func(got intent.Intent) {
				s.Equal("Prazeres", got.CategoryName())
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
			check: func(got intent.Intent) {
				s.Equal("2026-06", got.RefMonth())
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
			check: func(got intent.Intent) {
				s.Equal("oi bom dia", got.RawText())
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
			check: func(got intent.Intent) {
				s.Equal(int64(120000), got.AmountCents())
				s.Equal(6, got.Installments())
				s.Equal("nubank", got.CardHint())
			},
		},
		{
			name:    "list_transactions",
			llmJSON: `{"kind":"list_transactions","ref_month":"2026-06"}`,
			want:    intent.KindListTransactions,
			check: func(got intent.Intent) {
				s.Equal("2026-06", got.RefMonth())
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
			check: func(got intent.Intent) {
				s.Equal(int64(8000), got.AmountCents())
			},
		},
		{
			name:    "create_recurring_explicit_direction",
			llmJSON: `{"kind":"create_recurring","amount_cents":500000,"merchant":"salário","direction":"income","frequency":"monthly","day_of_month":5}`,
			want:    intent.KindCreateRecurring,
			check: func(got intent.Intent) {
				s.Equal("income", got.Direction())
				s.Equal("monthly", got.Frequency())
				s.Equal(5, got.DayOfMonth())
			},
		},
		{
			name:    "create_recurring_infers_outcome_default",
			llmJSON: `{"kind":"create_recurring","amount_cents":120000,"merchant":"aluguel","day_of_month":0}`,
			want:    intent.KindCreateRecurring,
			check: func(got intent.Intent) {
				s.Equal("outcome", got.Direction())
				s.Equal("monthly", got.Frequency())
				s.Equal(1, got.DayOfMonth())
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
		s.Run(tc.name, func() {
			uc := s.newSUT(tc.llmJSON, nil)
			out, err := uc.Execute(s.ctx, ParseInboundInput{
				UserID: uuid.New(),
				Text:   "qualquer texto",
			})
			s.Require().NoError(err)
			s.Equal(tc.want, out.Intent.Kind())
			if tc.check != nil {
				tc.check(out.Intent)
			}
		})
	}
}

func (s *ParseInboundSuite) TestExecuteCreateRecurringInfersIncomeFromText() {
	uc := s.newSUT(`{"kind":"create_recurring","amount_cents":500000,"merchant":"salário"}`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "todo mês recebo 5000 de salário",
	})
	s.Require().NoError(err)
	s.Equal(intent.KindCreateRecurring, out.Intent.Kind())
	s.Equal("income", out.Intent.Direction())
}

func (s *ParseInboundSuite) TestExecuteInvalidJSONFallback() {
	uc := s.newSUT(`not a json`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "preciso pagar a fatura",
	})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
	s.Equal("preciso pagar a fatura", out.Intent.RawText())
}

func (s *ParseInboundSuite) TestExecuteMissingKindFallback() {
	uc := s.newSUT(`{"amount_cents":100}`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "gastei algo",
	})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
}

func (s *ParseInboundSuite) TestExecuteDomainInvariantViolationFallback() {
	uc := s.newSUT(`{"kind":"log_expense","amount_cents":0}`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "gastei algo",
	})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
}

func (s *ParseInboundSuite) TestExecuteRecoversInstallmentsFromText() {
	uc := s.newSUT(`{"kind":"log_card_purchase","amount_cents":120000,"card_hint":"nubank"}`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: "comprei 1200 em 6x no nubank"})
	s.Require().NoError(err)
	s.Equal(intent.KindLogCardPurchase, out.Intent.Kind())
	s.Equal(6, out.Intent.Installments())
}

func (s *ParseInboundSuite) TestExecuteRecoversInstallmentsVariants() {
	cases := map[string]int{
		"parcelei 600 em 6 vezes no nubank": 6,
		"passei 900 no cartão em 3x":        3,
		"comprei em 12 parcelas":            12,
	}
	for text, want := range cases {
		s.Run(text, func() {
			uc := s.newSUT(`{"kind":"log_card_purchase","amount_cents":90000,"card_hint":"nubank"}`, nil)
			out, err := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: text})
			s.Require().NoError(err)
			s.Equal(intent.KindLogCardPurchase, out.Intent.Kind())
			s.Equal(want, out.Intent.Installments())
		})
	}
}

func (s *ParseInboundSuite) TestExecuteCardPurchaseWithoutInstallmentCueFallsBack() {
	uc := s.newSUT(`{"kind":"log_card_purchase","amount_cents":120000,"card_hint":"nubank"}`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: "comprei algo caro no cartão"})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
}

func (s *ParseInboundSuite) TestExecuteProviderErrorFallback() {
	uc := s.newSUT(``, errors.New("upstream timeout"))
	out, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "como tá meu cartão?",
	})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
	s.Equal("como tá meu cartão?", out.Intent.RawText())
}

func (s *ParseInboundSuite) TestExecuteForwardsJSONSchemaToInterpreter() {
	fi := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"how_am_i_doing"}`)}}
	uc, err := NewParseInbound(fi, 2000, fake.NewProvider())
	s.Require().NoError(err)

	_, err = uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "como tá meu mês?",
	})
	s.Require().NoError(err)
	s.Require().NotNil(fi.lastRequest.JSONSchema)
	s.Equal("mecontrola_parse_intent", fi.lastRequest.JSONSchema.Name)
	s.Empty(fi.lastRequest.Tools)
}

func (s *ParseInboundSuite) TestExecuteNoToolCallPropagatesDirectReply() {
	const reply = "Claro! Qual o valor que você gastou?"
	uc := s.newSUT(reply, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: "quero registrar um gasto"})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
	s.Equal(reply, out.DirectReply)
}

func (s *ParseInboundSuite) TestExecuteUnsupportedToolCallFallback() {
	fi := &fakeInterpreter{resp: interfaces.LLMResponse{
		ToolCalls: []interfaces.ToolCall{{ID: "call_x", FunctionName: "nonexistent_tool"}},
	}}
	uc, err := NewParseInbound(fi, 2000, fake.NewProvider())
	s.Require().NoError(err)

	out, err := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: "faça algo estranho"})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
	s.Empty(out.DirectReply)
}

func (s *ParseInboundSuite) TestExecuteUnknownKindStringFallback() {
	uc := s.newSUT(`{"kind":"bogus"}`, nil)
	out, err := uc.Execute(s.ctx, ParseInboundInput{
		UserID: uuid.New(),
		Text:   "texto",
	})
	s.Require().NoError(err)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
}

func (s *ParseInboundSuite) decodeFailureReasons(provider *fake.Provider) []string {
	metrics, ok := provider.Metrics().(*fake.FakeMetrics)
	s.Require().True(ok)
	counter := metrics.GetCounter("agent_intent_parse_decode_failed_total")
	s.Require().NotNil(counter)
	reasons := make([]string, 0)
	for _, v := range counter.GetValues() {
		for _, f := range v.Fields {
			if f.Key == "reason" {
				reasons = append(reasons, f.StringValue())
			}
		}
	}
	return reasons
}

func (s *ParseInboundSuite) TestInvalidJSONRecordsDecodeFailure() {
	provider := fake.NewProvider()
	uc, err := NewParseInbound(&fakeInterpreter{
		resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind": "log_expense", broken`)},
	}, 2000, provider)
	s.Require().NoError(err)

	out, execErr := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: "gastei 58 no ifood"})
	s.Require().NoError(execErr)
	s.Equal(intent.KindUnknown, out.Intent.Kind())
	s.Empty(out.DirectReply)
	s.Contains(s.decodeFailureReasons(provider), outcomeFallbackInvalid)
}

func (s *ParseInboundSuite) TestRefusalProseStaysDirectReply() {
	provider := fake.NewProvider()
	const reply = "Desculpe, não posso ajudar com isso."
	uc, err := NewParseInbound(&fakeInterpreter{
		resp: interfaces.LLMResponse{RawJSON: []byte(reply)},
	}, 2000, provider)
	s.Require().NoError(err)

	out, execErr := uc.Execute(s.ctx, ParseInboundInput{UserID: uuid.New(), Text: "faça algo"})
	s.Require().NoError(execErr)
	s.Equal(reply, out.DirectReply)
	s.NotContains(s.decodeFailureReasons(provider), outcomeFallbackInvalid)
}
