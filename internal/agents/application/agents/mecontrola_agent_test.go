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
	provider := llmmocks.NewProvider(s.T())
	provider.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Run(func(_ context.Context, req llm.Request) {
			s.Equal(mecontrolaAgentDefaultMaxTokens, req.MaxTokens)
		}).
		Return(llm.Response{Content: "ok"}, nil).
		Once()

	obs := fake.NewProvider()
	a := BuildMeControlaAgent(provider, nil, nil, obs)

	result, err := a.Execute(s.ctx, agent.Request{
		AgentID:  MecontrolaAgentID,
		Messages: []llm.Message{{Role: "user", Content: "oi"}},
	})

	s.NoError(err)
	s.Equal("ok", result.Content)
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

	provider := llmmocks.NewProvider(s.T())
	provider.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Run(func(_ context.Context, req llm.Request) {
			s.GreaterOrEqual(req.MaxTokens, 1536)
		}).
		Return(llm.Response{Content: fullResponse}, nil).
		Once()

	obs := fake.NewProvider()
	a := BuildMeControlaAgent(provider, nil, nil, obs)

	result, err := a.Execute(s.ctx, agent.Request{
		AgentID:  MecontrolaAgentID,
		Messages: []llm.Message{{Role: "user", Content: "quero configurar meu orçamento"}},
	})

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
}
