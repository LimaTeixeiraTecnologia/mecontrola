package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type CategoryWithoutToolGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCategoryWithoutToolGuardSuite(t *testing.T) {
	suite.Run(t, new(CategoryWithoutToolGuardSuite))
}

func (s *CategoryWithoutToolGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CategoryWithoutToolGuardSuite) TestInspect() {
	scenarios := []struct {
		name   string
		out    agent.Result
		expect func(decision GuardDecision)
	}{
		{
			name: "pergunta de categoria raiz fabricada sem tool e substituida por falha honesta",
			out: agent.Result{
				Content: "Em qual categoria isso se encaixa? 📂\n1. *Conhecimento*\n2. *Custo Fixo*\n\nResponda o número ou o nome. 🙂",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
				s.Equal("guard category_without_tool: pergunta de categoria fabricada pelo LLM sem tool call", decision.Result.FailureReason)
			},
		},
		{
			name: "pergunta de subcategoria fabricada sem tool e substituida por falha honesta",
			out: agent.Result{
				Content: "Dentro de *Custo Fixo*, qual subcategoria? 📂\n1. *Açougue*\n2. *Água*",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "pergunta de folha fabricada sem tool e substituida por falha honesta",
			out: agent.Result{
				Content: "Qual se encaixa melhor? 📂\n1. *Custo Fixo > Transporte por Aplicativo Recorrente*\n2. *Prazeres > Transporte de Lazer*",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "pergunta fabricada com redacao parafraseada mas estrutura do catalogo",
			out: agent.Result{
				Content: "Qual é a categoria para esse gasto? 📂\n1. *Conhecimento*\n2. *Custo Fixo*\n3. *Liberdade Financeira*\n4. *Metas*\n5. *Prazeres*\n\nResponda o número ou o nome. 🙂",
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "pergunta de categoria com tool auxiliar sem verbatim e substituida por falha honesta",
			out: agent.Result{
				Content:   "Em qual categoria isso se encaixa? 📂",
				ToolCalls: []agent.ToolCallRecord{{Tool: "resolve_card_by_nickname", Outcome: agent.ToolCallOutcomeSuccess}},
			},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(successWithoutToolFallbackMessage, decision.Result.Content)
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
			},
		},
		{
			name: "pergunta de categoria com verbatim legitimo passa adiante",
			out: agent.Result{
				Content: "texto livre do modelo",
				ToolCalls: []agent.ToolCallRecord{{
					Tool:    "register_expense",
					Outcome: agent.ToolCallOutcomeSuccess,
					Content: `{"clarifyPrompt":"Em qual categoria isso se encaixa? 📂"}`,
				}},
			},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
		{
			name: "resposta sem marcador de categoria passa adiante",
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
			guard := NewCategoryWithoutToolGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.out)
			scenario.expect(decision)
		})
	}
}
