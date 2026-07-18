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
			},
		},
		{
			name: "bloco de confirmacao com tool chamada passa adiante",
			out: agent.Result{
				Content:   "✅ Encontrei este lançamento. Posso registrar?",
				ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
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
