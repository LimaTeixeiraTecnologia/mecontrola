package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
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
