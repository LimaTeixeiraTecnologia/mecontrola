package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type CardProvenanceGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCardProvenanceGuardSuite(t *testing.T) {
	suite.Run(t, new(CardProvenanceGuardSuite))
}

func (s *CardProvenanceGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CardProvenanceGuardSuite) TestName() {
	guard := NewCardProvenanceGuard()
	s.Equal("card_provenance", guard.Name())
}

func (s *CardProvenanceGuardSuite) TestInspect_NonCreditCardPaymentMethods() {
	nonCardMethods := []string{
		"pix",
		"cash",
		"boleto",
		"ted",
		"debit_card",
		"debit_in_account",
		"vale_refeicao",
		"vale_alimentacao",
	}

	for _, method := range nonCardMethods {
		s.Run("register_expense_"+method, func() {
			guard := NewCardProvenanceGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
				Content: "Confirma lançamento?",
				ToolCalls: []agent.ToolCallRecord{
					{
						Tool:          "register_expense",
						Outcome:       agent.ToolCallOutcomeSuccess,
						Content:       `{"outcome":"clarify","message":"Confirma lançamento?"}`,
						ArgumentsJSON: map[string]any{"paymentMethod": method},
					},
				},
			})
			s.False(decision.Handled)
			s.Equal("", decision.Result.Content)
		})
	}
}

func (s *CardProvenanceGuardSuite) TestInspect_RegisterExpenseCreditCard_WithoutResolution() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: "Lançamento registrado.",
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:          "register_expense",
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       `{"outcome":"routed"}`,
				ArgumentsJSON: map[string]any{"paymentMethod": "credit_card"},
			},
		},
	})
	s.True(decision.Handled)
	s.Equal(cardProvenanceFallbackMessage, decision.Result.Content)
	s.Contains(decision.Result.Content, "💳")
	s.NotContains(decision.Result.Content, "cartão")
	s.Equal(agent.ToolOutcomeClarify, decision.Result.ToolOutcome)
}

func (s *CardProvenanceGuardSuite) TestInspect_CreateRecurrenceCreditCard_WithoutResolution() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: "Recorrência solicitada.",
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:          "create_recurrence",
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       `{"outcome":"clarify"}`,
				ArgumentsJSON: map[string]any{"paymentMethod": "credit_card"},
			},
		},
	})
	s.True(decision.Handled)
}

func (s *CardProvenanceGuardSuite) TestInspect_CreateRecurrencePix_WithoutResolution() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: "Recorrência solicitada.",
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:          "create_recurrence",
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       `{"outcome":"clarify"}`,
				ArgumentsJSON: map[string]any{"paymentMethod": "pix"},
			},
		},
	})
	s.False(decision.Handled)
}

func (s *CardProvenanceGuardSuite) TestInspect_QueryCardInvoice_WithoutResolution() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: "Fatura consultada.",
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:    "query_card_invoice",
				Outcome: agent.ToolCallOutcomeSuccess,
				Content: `{}`,
			},
		},
	})
	s.True(decision.Handled)
}

func (s *CardProvenanceGuardSuite) TestInspect_WithPriorResolution() {
	scenarios := []struct {
		name  string
		calls []agent.ToolCallRecord
	}{
		{
			name: "resolve_card antes de register_expense credit_card",
			calls: []agent.ToolCallRecord{
				{Tool: "resolve_card", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"found":true,"cardId":"card-1"}`},
				{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"outcome":"routed"}`, ArgumentsJSON: map[string]any{"paymentMethod": "credit_card"}},
			},
		},
		{
			name: "list_cards antes de query_card_invoice",
			calls: []agent.ToolCallRecord{
				{Tool: "list_cards", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"cards":[]}`},
				{Tool: "query_card_invoice", Outcome: agent.ToolCallOutcomeSuccess, Content: `{}`},
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewCardProvenanceGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
				Content:   "Ok.",
				ToolCalls: scenario.calls,
			})
			s.False(decision.Handled)
		})
	}
}

func (s *CardProvenanceGuardSuite) TestInspect_ResolveCardNotFoundForcesCardClarification() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: "Não encontrei esse item.",
		ToolCalls: []agent.ToolCallRecord{
			{Tool: "resolve_card", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"found":false}`},
		},
	})
	s.True(decision.Handled)
	s.Contains(decision.Result.Content, "💳")
	s.Equal(agent.ToolOutcomeClarify, decision.Result.ToolOutcome)
}

func (s *CardProvenanceGuardSuite) TestInspect_NoToolCalls() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{Content: "Como posso ajudar?"})
	s.False(decision.Handled)
}

func (s *CardProvenanceGuardSuite) TestInspect_DoesNotOverrideVerbatimForNonCreditCard() {
	verbatim := "Confirma o lançamento de R$ 50,00 no supermercado via pix?"
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: verbatim,
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:          "register_expense",
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       `{"outcome":"clarify","message":"` + verbatim + `"}`,
				ArgumentsJSON: map[string]any{"paymentMethod": "pix"},
			},
		},
	})
	s.False(decision.Handled)
	s.Equal("", decision.Result.Content)
}

func (s *CardProvenanceGuardSuite) TestInspect_PaymentMethodAsBytes() {
	guard := NewCardProvenanceGuard()
	decision := guard.Inspect(s.ctx, agent.Request{}, agent.Result{
		Content: "Lançamento registrado.",
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:          "register_expense",
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       `{"outcome":"routed"}`,
				ArgumentsJSON: map[string]any{"paymentMethod": []byte("credit_card")},
			},
		},
	})
	s.True(decision.Handled)
}
