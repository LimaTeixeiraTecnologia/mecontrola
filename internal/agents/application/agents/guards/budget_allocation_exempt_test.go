package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type BudgetAllocationExemptSuite struct {
	suite.Suite
	ctx context.Context
}

func TestBudgetAllocationExemptSuite(t *testing.T) {
	suite.Run(t, new(BudgetAllocationExemptSuite))
}

func (s *BudgetAllocationExemptSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *BudgetAllocationExemptSuite) requestWith(message string) agent.Request {
	return agent.Request{Messages: []llm.Message{{Role: "user", Content: message}}}
}

func (s *BudgetAllocationExemptSuite) TestMultiItemGuardExemption() {
	scenarios := []struct {
		name    string
		message string
		blocked bool
	}{
		{
			name:    "producao: distribuicao em reais com 5 categorias nao bloqueia",
			message: "Ah, eu quero fazer ajuste, ó. Eu quero o custo fixo R$ 2.000, conhecimento R$ 1.000, prazeres R$ 1.000, metas R$ 250 e liberdade financeira R$ 750.",
			blocked: false,
		},
		{
			name:    "producao: edicao de distribuicao com 5 categorias nao bloqueia",
			message: "Eu estou editando o meu orçamento, a distribuição do meu orçamento entre as categorias. Eu gostaria que fosse colocado R$ 2.500 no custo fixo, conhecimento R$ 200, prazeres R$ 1.300, metas R$ 500 e liberdade financeira R$ 500.",
			blocked: false,
		},
		{
			name:    "distribuicao em porcentagem nao bloqueia",
			message: "custo fixo 50%, prazeres 20%, metas 30%",
			blocked: false,
		},
		{
			name:    "regressao: multi lancamento genuino continua bloqueando",
			message: "gastei 30 no uber e 50 no mercado",
			blocked: true,
		},
		{
			name:    "regressao: tres lancamentos genuinos continua bloqueando",
			message: "paguei 20 reais no almoço, 15 reais no café e 40 reais na farmácia",
			blocked: true,
		},
		{
			name:    "regressao: uma unica categoria com dois valores continua bloqueando",
			message: "gastei 30 em prazeres e 50 tambem",
			blocked: true,
		},
	}

	guard := NewMultiItemGuard()
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := guard.Inspect(s.ctx, s.requestWith(scenario.message))
			s.Equal(scenario.blocked, decision.Handled)
			if scenario.blocked {
				s.Equal(workflows.MultiItemOrientationMessage, decision.Result.Content)
			}
		})
	}
}

func (s *BudgetAllocationExemptSuite) TestIsBudgetAllocationIntent() {
	scenarios := []struct {
		name    string
		message string
		expect  bool
	}{
		{name: "cinco categorias", message: "custo fixo 2000, conhecimento 1000, prazeres 1000, metas 250 e liberdade financeira 750", expect: true},
		{name: "duas categorias", message: "prazeres 300 e metas 200", expect: true},
		{name: "uma categoria so", message: "gastei 300 em prazeres", expect: false},
		{name: "nenhuma categoria", message: "gastei 30 no uber e 50 no mercado", expect: false},
		{name: "variante custos fixos plural conta uma vez", message: "custos fixos 2000 e custo fixo 3000", expect: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, IsBudgetAllocationIntent(scenario.message))
		})
	}
}

func (s *BudgetAllocationExemptSuite) TestBudgetDistributionShortcutAcceptsReais() {
	scenarios := []struct {
		name    string
		message string
		expect  bool
	}{
		{
			name:    "producao: valores em reais agora entram no atalho",
			message: "Ah, eu quero fazer ajuste, ó. Eu quero o custo fixo R$ 2.000, conhecimento R$ 1.000, prazeres R$ 1.000, metas R$ 250 e liberdade financeira R$ 750.",
			expect:  true,
		},
		{
			name:    "regressao: porcentagem continua entrando",
			message: "Quero mudar a distribuição do orçamento de junho para: Metas 20%, Prazeres 10%, Custo Fixo 45%, Conhecimento 5%, Liberdade Financeira 20%",
			expect:  true,
		},
		{
			name:    "regressao: mensagem sem categoria nao entra",
			message: "gastei 30 no uber e 50 no mercado",
			expect:  false,
		},
		{
			name:    "regressao: uma categoria sem percentual nao entra",
			message: "como estao meus prazeres esse mes",
			expect:  false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			_, ok := parseBudgetDistributionShortcut(scenario.message)
			s.Equal(scenario.expect, ok)
		})
	}
}

func (s *BudgetAllocationExemptSuite) TestBudgetSlotWithoutToolGuard() {
	scenarios := []struct {
		name      string
		content   string
		toolCalls []agent.ToolCallRecord
		handled   bool
	}{
		{
			name:    "producao: pergunta de distribuicao fabricada sem tool e bloqueada",
			content: "Stef, ótimo! Agora, precisamos definir a distribuição do seu orçamento de R$ 5.000,00 entre as categorias. Qual você prefere?",
			handled: true,
		},
		{
			name:    "producao: pergunta de valor total fabricada sem tool e bloqueada",
			content: "Qual é o valor total que você gostaria de definir para o novo orçamento?",
			handled: true,
		},
		{
			name:      "com tool de orcamento chamada nao bloqueia",
			content:   "Agora, precisamos definir a distribuição do seu orçamento entre as categorias?",
			toolCalls: []agent.ToolCallRecord{{Tool: "adjust_allocation", Outcome: agent.ToolCallOutcomeSuccess}},
			handled:   false,
		},
		{
			name:    "resposta informativa sem pergunta nao bloqueia",
			content: "Seu orçamento total é de R$ 5.000,00 e está distribuído entre as categorias.",
			handled: false,
		},
		{
			name:    "resposta sem mencionar orcamento nao bloqueia",
			content: "Em qual categoria isso se encaixa?",
			handled: false,
		},
	}

	guard := NewBudgetSlotWithoutToolGuard()
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			out := agent.Result{Content: scenario.content, ToolCalls: scenario.toolCalls}
			decision := guard.Inspect(s.ctx, agent.Request{}, out)
			s.Equal(scenario.handled, decision.Handled)
			if scenario.handled {
				s.Equal(agent.ToolOutcomeUsecaseError, decision.Result.ToolOutcome)
				s.True(decision.Retryable)
			}
		})
	}
}

func (s *BudgetAllocationExemptSuite) TestGuardRoteiaFrasesReaisDeProducaoParaAdjustAllocation() {
	scenarios := []struct {
		name    string
		message string
	}{
		{
			name:    "producao: ajuste com 5 valores em reais",
			message: "Ah, eu quero fazer ajuste, ó. Eu quero o custo fixo R$ 2.000, conhecimento R$ 1.000, prazeres R$ 1.000, metas R$ 250 e liberdade financeira R$ 750.",
		},
		{
			name:    "producao: edicao de distribuicao com 5 valores em reais",
			message: "Eu estou editando o meu orçamento, a distribuição do meu orçamento entre as categorias. Eu gostaria que fosse colocado R$ 2.500 no custo fixo, conhecimento R$ 200, prazeres R$ 1.300, metas R$ 500 e liberdade financeira R$ 500.",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			adjust := &stubBudgetTool{
				id:  "adjust_allocation",
				raw: []byte(`{"outcome":"started","confirmationPrompt":"📊 Confirma a nova distribuição?"}`),
			}
			total := &stubBudgetTool{id: "edit_budget_total"}
			guard := NewBudgetWriteShortcutGuard(adjust, total)

			decision := guard.Inspect(s.ctx, s.requestWith(scenario.message))

			s.True(decision.Handled, "guard de orçamento deve assumir a frase")
			s.True(adjust.invoked, "adjust_allocation deve ser invocada")
			s.False(total.invoked)
			s.Equal("📊 Confirma a nova distribuição?", decision.Result.Content)
			s.Equal(agent.ToolOutcomeRouted, decision.Result.ToolOutcome)
		})
	}
}
