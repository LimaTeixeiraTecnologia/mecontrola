package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type ConfirmationWithoutToolGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestConfirmationWithoutToolGuardSuite(t *testing.T) {
	suite.Run(t, new(ConfirmationWithoutToolGuardSuite))
}

func (s *ConfirmationWithoutToolGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ConfirmationWithoutToolGuardSuite) TestInspect() {
	scenarios := []struct {
		name   string
		out    agent.Result
		expect func(decision GuardDecision)
	}{
		{
			name: "bloco de confirmacao fabricado sem tool e substituido por falha honesta",
			out: agent.Result{
				Content: "✅ Encontrei esta despesa:\n\n💰 Valor: R$ 55,00\n📂 Categoria: custos fixos > serviços\n\nPosso registrar?",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
				s.Equal("guard confirmation_without_tool: pedido de confirmação fabricado pelo LLM sem tool call", decision.Result.FailureReason)
			},
		},
		{
			name: "bloco de confirmacao com tool auxiliar sem verbatim e substituido por falha honesta",
			out: agent.Result{
				Content:   "✅ Encontrei este lançamento. Posso registrar?",
				ToolCalls: []agent.ToolCallRecord{{Tool: "resolve_card_by_nickname", Outcome: agent.ToolCallOutcomeSuccess}},
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "bloco de confirmacao com verbatim de tool de escrita passa adiante",
			out: agent.Result{
				Content: "✅ Encontrei este lançamento. Posso registrar?",
				ToolCalls: []agent.ToolCallRecord{{
					Tool:    "register_expense",
					Outcome: agent.ToolCallOutcomeSuccess,
					Content: `{"message":"✅ Encontrei este lançamento. Posso registrar?"}`,
				}},
			},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "pergunta canned de confirmacao destrutiva fabricada sem tool e substituida por falha honesta",
			out: agent.Result{
				Content: "⚠️ Você confirma esta operação?\n\nEsta recorrência será excluída.\n\nResponda sim para confirmar ou não para cancelar.",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "texto canned de cancelamento fabricado sem tool e substituido por falha honesta",
			out: agent.Result{
				Content: "🚫 Operação cancelada conforme solicitado.",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "reprompt canned de sim ou nao fabricado sem tool e substituido por falha honesta",
			out: agent.Result{
				Content: "Não entendi. Por favor, responda apenas sim ou não para confirmar a operação.",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "pergunta canned com verbatim de tool destrutiva passa adiante",
			out: agent.Result{
				Content: "⚠️ Você confirma esta operação?\n\nEsta recorrência será atualizada.",
				ToolCalls: []agent.ToolCallRecord{{
					Tool:    "update_recurrence",
					Outcome: agent.ToolCallOutcomeSuccess,
					Content: `{"message":"⚠️ Você confirma esta operação?\n\nEsta recorrência será atualizada."}`,
				}},
			},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "resposta sem marcador de confirmacao passa adiante",
			out: agent.Result{
				Content: "Prontinho! ✅",
			},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewConfirmationWithoutToolGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.out)
			scenario.expect(decision)
		})
	}
}
