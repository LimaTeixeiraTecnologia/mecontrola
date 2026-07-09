package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/formatting"
)

type MecontrolaAgentBuilderSuite struct {
	suite.Suite
	ctx context.Context
}

func TestMecontrolaAgentBuilderSuite(t *testing.T) {
	suite.Run(t, new(MecontrolaAgentBuilderSuite))
}

func (s *MecontrolaAgentBuilderSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MecontrolaAgentBuilderSuite) TestBuildMeControlaAgent_HasCorrectID() {
	type dependencies struct{}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(id string)
	}{
		{
			name:         "deve retornar ID correto do agente mecontrola",
			dependencies: dependencies{},
			expect: func(id string) {
				s.Equal(MecontrolaAgentID, id)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			provider := llmmocks.NewProvider(s.T())
			obs := fake.NewProvider()
			a := BuildMeControlaAgent(provider, nil, nil, obs)
			scenario.expect(a.ID())
		})
	}
}

func (s *MecontrolaAgentBuilderSuite) TestBuildMeControlaAgent_HasInstructions() {
	type dependencies struct{}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(instructions string)
	}{
		{
			name:         "deve ter instructions nao vazias em pt-br",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.NotEmpty(instructions)
				s.Contains(instructions, "português do Brasil")
			},
		},
		{
			name:         "deve conter identidade de parceiro financeiro",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "parceiro financeiro")
			},
		},
		{
			name:         "deve conter regras de comunicacao (uma pergunta por mensagem)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "UMA pergunta por mensagem")
			},
		},
		{
			name:         "deve conter emojis esperados na instrucao",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "✅")
				s.Contains(instructions, "💰")
				s.Contains(instructions, "📊")
				s.Contains(instructions, "🎯")
				s.Contains(instructions, "⚠️")
				s.Contains(instructions, "💡")
			},
		},
		{
			name:         "deve reforcar negrito compativel com whatsapp",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "use negrito apenas com *asterisco simples*")
				s.Contains(instructions, "nunca use **texto**")
			},
		},
		{
			name:         "deve exigir emojis em resumo e confirmacao final",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "Todo resumo de onboarding ou orçamento deve usar 📊")
				s.Contains(instructions, "Toda pergunta final de confirmação deve usar ✅ ou 🎯")
			},
		},
		{
			name:         "deve conter regra de pendencia conversacional",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA ABSOLUTA DE PENDÊNCIA CONVERSACIONAL")
				s.Contains(instructions, "outcome=clarify")
				s.Contains(instructions, "APENAS UMA pergunta")
			},
		},
		{
			name:         "deve conter template de cancelamento",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "Tudo certo, o registro foi cancelado.")
			},
		},
		{
			name:         "deve conter template de expiracao",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "O registro expirou. Para registrar, envie a informação completa novamente.")
			},
		},
		{
			name:         "deve conter template de erro de registro",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "Não consegui registrar. Tente novamente em breve.")
			},
		},
		{
			name:         "deve proibir mencao a infraestrutura interna",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "NUNCA mencione")
				s.NotContains(instructions, "workflow_id")
				s.NotContains(instructions, "correlation_key")
				s.NotContains(instructions, "resource_id")
				s.NotContains(instructions, "thread_id")
			},
		},
		{
			name:         "deve orientar multiplos candidatos com lista numerada",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "lista numerada com NOMES de categoria")
				s.Contains(instructions, "Qual se encaixa melhor?")
			},
		},
		{
			name:         "deve delegar confirmacao de escrita financeira exclusivamente ao sistema",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "responsabilidade EXCLUSIVA do sistema (gate do workflow) — NUNCA do LLM")
				s.Contains(instructions, "Você NUNCA formula, redige ou improvisa uma pergunta de confirmação própria")
			},
		},
		{
			name:         "deve conter os cinco campos obrigatorios (RF-01)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA ABSOLUTA DE CAMPOS OBRIGATÓRIOS")
				s.Contains(instructions, "cinco campos")
				s.Contains(instructions, "(1) data que a transação ocorreu")
				s.Contains(instructions, "(2) categoria raiz válida")
				s.Contains(instructions, "(3) subcategoria folha ligada à raiz")
				s.Contains(instructions, "(4) descrição")
				s.Contains(instructions, "(5) valor positivo em centavos")
			},
		},
		{
			name:         "deve proibir invencao de campo sem evidencia (RF-21)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "NUNCA invente, estime ou infira campo sem evidência")
				s.Contains(instructions, "NUNCA infira uma nova transação a partir de memória")
				s.Contains(instructions, "Informação incompleta ou ambígua → pedir esclarecimento")
			},
		},
		{
			name:         "deve instruir repasse de data cru sem conversao (RF-07, RF-09)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA ABSOLUTA DE DATA (occurredAt)")
				s.Contains(instructions, "texto de data CRU em occurredAt")
				s.Contains(instructions, "o sistema converte; o agente NÃO converte")
				s.Contains(instructions, "semana passada")
				s.Contains(instructions, "mês passado")
				s.Contains(instructions, "peça ao usuário uma data específica")
			},
		},
		{
			name:         "deve conter fronteira multi-item verbatim (RF-16)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA ABSOLUTA DE LANÇAMENTO ÚNICO")
				s.Contains(instructions, "UMA transação por mensagem")
				s.Contains(instructions, "Percebi mais de um lançamento na mesma mensagem")
				s.Contains(instructions, "registro um de cada vez")
				s.Contains(instructions, "NÃO registre nem chame nenhuma ferramenta de escrita quando detectar múltiplos")
			},
		},
		{
			name:         "deve instruir que ponto e separador de milhar e nao dispara multiplos lancamentos (RF-19/RF-20/RF-21)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "separador de milhar")
				s.Contains(instructions, "R$ 1.234,56")
				s.Contains(instructions, "R$ 13.874,40")
				s.Contains(instructions, "ignore pontos e vírgulas internos a um número monetário")
				s.Contains(instructions, "Percebi mais de um lançamento na mesma mensagem")
				s.Contains(instructions, "registro um de cada vez")
			},
		},
		{
			name:         "deve proibir inferencia de forma de pagamento e exigir pergunta com exemplos (RF-29/RF-30/RF-31/RF-32)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA ABSOLUTA DE FORMA DE PAGAMENTO")
				s.Contains(instructions, "NUNCA assuma, infira ou invente a forma de pagamento")
				s.Contains(instructions, "dinheiro\" NÃO é padrão nem suposição válida")
				s.Contains(instructions, "Como você pagou? Ex.: dinheiro, pix, débito, crédito, boleto, vale-refeição")
				s.Contains(instructions, "Receita (register_income) NUNCA pergunta forma de pagamento")
			},
		},
		{
			name:         "deve conter instrucao de repasse de occurredAt na chamada de ferramenta",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "texto de data CRU em occurredAt")
			},
		},
		{
			name:         "deve conter secao de consultas financeiras C1-C7",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "Consultas Financeiras (C1–C7)")
				s.Contains(instructions, "MATRIZ DE ROTEAMENTO — CONSULTAS")
			},
		},
		{
			name:         "deve conter roteamento deterministico C1 com query_month e query_plan",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "C1")
				s.Contains(instructions, "query_month E query_plan")
				s.Contains(instructions, "America/Sao_Paulo")
			},
		},
		{
			name:         "deve conter roteamento C4 resolve_card seguido de query_card_invoice",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "C4")
				s.Contains(instructions, "resolve_card")
				s.Contains(instructions, "query_card_invoice")
			},
		},
		{
			name:         "deve conter roteamento C5 com query_month limit=1 e get_transaction",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "C5")
				s.Contains(instructions, "query_month com limit=1")
				s.Contains(instructions, "get_transaction")
			},
		},
		{
			name:         "deve conter roteamento C6 com limit padrao 5",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "C6")
				s.Contains(instructions, "limit=5")
			},
		},
		{
			name:         "deve conter mapa slug para nome das 5 raizes",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "custo-fixo")
				s.Contains(instructions, "conhecimento")
				s.Contains(instructions, "prazeres")
				s.Contains(instructions, "metas")
				s.Contains(instructions, "liberdade-financeira")
				s.Contains(instructions, "Liberdade Financeira")
			},
		},
		{
			name:         "deve conter regra de formatacao de valores centavos para reais",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA DE FORMATAÇÃO DE VALORES")
				s.Contains(instructions, "123450 → R$ 1.234,50")
				s.Contains(instructions, "RF-22")
			},
		},
		{
			name:         "deve conter regra C5 de categoria best-effort e subcategoryNameSnapshot",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "subcategoryNameSnapshot")
				s.Contains(instructions, "categoryNameSnapshot > subcategoryNameSnapshot")
				s.Contains(instructions, "best-effort")
			},
		},
		{
			name:         "deve conter regra de mes vazio com retrocesso (RF-07a)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA DE MÊS VAZIO")
				s.Contains(instructions, "mês anterior")
			},
		},
		{
			name:         "deve conter regra de alertas em C2/C3/C7 (RF-08a)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA DE ALERTAS EM C2/C3/C7")
				s.Contains(instructions, "Nenhum alerta ativo. ✅")
			},
		},
		{
			name:         "deve conter regra C7 orçamento completo com plannedCents nulo",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA C7 — ORÇAMENTO COMPLETO")
				s.Contains(instructions, "*Sem limite definido*")
				s.Contains(instructions, "totalPlannedCents")
				s.Contains(instructions, "totalSpentCents")
			},
		},
		{
			name:         "deve conter guard de cardId exclusivamente de resolve_card ou list_cards (RF-32a)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA GUARD DE cardId")
				s.Contains(instructions, "EXCLUSIVAMENTE do retorno de resolve_card ou list_cards")
				s.Contains(instructions, "NUNCA use um cardId proveniente de texto do usuário")
			},
		},
		{
			name:         "deve conter regra de ambiguidade de cartao com found=false",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "REGRA DE AMBIGUIDADE DE CARTÃO")
				s.Contains(instructions, "found=false")
			},
		},
		{
			name:         "deve conter mensagens de ausencia e erro verbatim para consultas",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "Você ainda não tem um orçamento para")
				s.Contains(instructions, "Não encontrei fatura para o cartão")
				s.Contains(instructions, "Não há lançamentos em")
				s.Contains(instructions, "Não consegui consultar agora. Tente novamente em breve.")
			},
		},
		{
			name:         "deve conter regra de follow-up sem resposta de memoria (RF-26/RF-27)",
			dependencies: dependencies{},
			expect: func(instructions string) {
				s.Contains(instructions, "FOLLOW-UP")
				s.Contains(instructions, "nunca responda de memória")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			provider := llmmocks.NewProvider(s.T())
			obs := fake.NewProvider()
			a := BuildMeControlaAgent(provider, nil, nil, obs)
			scenario.expect(a.Instructions())
		})
	}
}

func (s *MecontrolaAgentBuilderSuite) TestBuildMeControlaAgent_DefaultMaxTokensApplied() {
	type dependencies struct {
		provider *llmmocks.Provider
	}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result agent.Result, err error)
	}{
		{
			name: "deve aplicar max tokens padrao na request ao llm",
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					provider := llmmocks.NewProvider(s.T())
					provider.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Run(func(_ context.Context, req llm.Request) {
							s.Equal(mecontrolaAgentDefaultMaxTokens, req.MaxTokens)
						}).
						Return(llm.Response{Content: "ok"}, nil).
						Once()
					return provider
				}(),
			},
			expect: func(result agent.Result, err error) {
				s.NoError(err)
				s.Equal("ok", result.Content)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			obs := fake.NewProvider()
			a := BuildMeControlaAgent(scenario.dependencies.provider, nil, nil, obs)
			result, err := a.Execute(s.ctx, agent.Request{
				AgentID:  MecontrolaAgentID,
				Messages: []llm.Message{{Role: "user", Content: "oi"}},
			})
			scenario.expect(result, err)
		})
	}
}

func (s *MecontrolaAgentBuilderSuite) TestBuildMeControlaAgent_DefaultMaxTokensCoversOnboardingResponse() {
	s.GreaterOrEqual(mecontrolaAgentDefaultMaxTokens, 1536)
}

func (s *MecontrolaAgentBuilderSuite) TestBuildMeControlaAgent_OnboardingSummaryNotTruncatedWithEmojis() {
	fullResponse := "1. *Custo Fixo*: 💰 Esta categoria abrange todas as despesas que você tem todo mês e que não podem ser evitadas, como aluguel, contas de luz, água e internet.\n\n" +
		"2. *Conhecimento*: 💡 Aqui você destina uma parte do seu orçamento para investir em sua educação e desenvolvimento pessoal, como cursos, livros e workshops.\n\n" +
		"3. *Prazeres*: Esta categoria é dedicada ao seu bem-estar e lazer, como sair para jantar, viajar ou praticar hobbies.\n\n" +
		"4. *Metas*: 🎯 Nesta categoria você define um percentual do seu orçamento para alcançar objetivos financeiros específicos, garantindo que você está progredindo.\n\n" +
		"5. *Liberdade Financeira*: Aqui você constrói seu patrimônio e caminha para a independência financeira.\n\n" +
		"*Resumo do Onboarding:*\n" +
		"- *Renda Mensal:* R$8.000,00\n" +
		"- *Distribuição de Despesas:*\n" +
		"  - Conhecimento: 20%\n" +
		"  - Custo Fixo: 30%\n" +
		"  - Liberdade Financeira: 20%\n" +
		"  - Metas: 10%\n" +
		"  - Prazeres: 20%\n\n" +
		"Por favor, confirme se deseja ativar o orçamento com as informações acima."

	type dependencies struct {
		provider *llmmocks.Provider
	}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result agent.Result, err error)
	}{
		{
			name: "deve preservar resumo de onboarding com emojis sem truncar",
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					provider := llmmocks.NewProvider(s.T())
					provider.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Run(func(_ context.Context, req llm.Request) {
							s.GreaterOrEqual(req.MaxTokens, 1536)
						}).
						Return(llm.Response{Content: fullResponse}, nil).
						Once()
					return provider
				}(),
			},
			expect: func(result agent.Result, err error) {
				s.NoError(err)
				s.False(result.TruncatedByLength)

				normalized := formatting.NormalizeOutboundText(result.Content)

				s.NotContains(normalized, "**")
				s.Contains(normalized, "📊")
				s.True(strings.Contains(normalized, "✅") || strings.Contains(normalized, "🎯"))
				for _, category := range []string{"*Custo Fixo*", "*Conhecimento*", "*Prazeres*", "*Metas*", "*Liberdade Financeira*"} {
					s.Contains(normalized, category)
				}
				s.Contains(normalized, "✅ Por favor, confirme")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			obs := fake.NewProvider()
			a := BuildMeControlaAgent(scenario.dependencies.provider, nil, nil, obs)
			result, err := a.Execute(s.ctx, agent.Request{
				AgentID:  MecontrolaAgentID,
				Messages: []llm.Message{{Role: "user", Content: "quero configurar meu orçamento"}},
			})
			scenario.expect(result, err)
		})
	}
}
