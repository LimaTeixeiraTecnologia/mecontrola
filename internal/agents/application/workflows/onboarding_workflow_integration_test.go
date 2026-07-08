//go:build integration

package workflows_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func buildRealLLMProvider(t *testing.T) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		t.Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")
	}
	baseURL := "https://openrouter.ai"
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)
	model := os.Getenv("AGENT_HARNESS_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	return llm.NewOpenRouterProvider(client, llm.Config{
		Model:          model,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:         "mecontrola-integration-test",
		MaxTokens:      1536,
		Temperature:    0,
		RequestTimeout: 30 * time.Second,
	}, fake.NewProvider())
}

func onboardingSuggestReturn(income int64) []interfaces.AllocationCents {
	bp := map[string]int{
		"expense.custo_fixo":           4000,
		"expense.conhecimento":         1000,
		"expense.prazeres":             1000,
		"expense.metas":                1000,
		"expense.liberdade_financeira": 3000,
	}
	slugs := []string{
		"expense.custo_fixo",
		"expense.conhecimento",
		"expense.prazeres",
		"expense.metas",
		"expense.liberdade_financeira",
	}
	out := make([]interfaces.AllocationCents, 0, len(slugs))
	for _, slug := range slugs {
		out = append(out, interfaces.AllocationCents{
			RootSlug:     slug,
			BasisPoints:  bp[slug],
			PlannedCents: income * int64(bp[slug]) / 10000,
		})
	}
	return out
}

type OnboardingWorkflowRealLLMSuite struct {
	suite.Suite
	provider llm.Provider
	model    string
}

func TestOnboardingWorkflowRealLLMSuite(t *testing.T) {
	suite.Run(t, new(OnboardingWorkflowRealLLMSuite))
}

func (s *OnboardingWorkflowRealLLMSuite) SetupTest() {
	s.provider = buildRealLLMProvider(s.T())
	s.model = os.Getenv("AGENT_HARNESS_MODEL")
	if s.model == "" {
		s.model = "openai/gpt-4o-mini"
	}
}

func (s *OnboardingWorkflowRealLLMSuite) TestGoalValueCombinedExtractionGate() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs)

	type args struct {
		resumeText  string
		refusalText string
	}
	type expected struct {
		goalPresent bool
		valueCents  int64
	}

	scenarios := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name:     "junto-mascarado-virgula",
			args:     args{resumeText: "comprar uma casa, meta de R$ 400.000,00"},
			expected: expected{goalPresent: true, valueCents: 40000000},
		},
		{
			name:     "junto-digitos-puros",
			args:     args{resumeText: "montar uma reserva de 400000"},
			expected: expected{goalPresent: true, valueCents: 40000000},
		},
		{
			name:     "junto-coloquial-mil",
			args:     args{resumeText: "reserva de emergência de 10 mil reais"},
			expected: expected{goalPresent: true, valueCents: 1000000},
		},
		{
			name:     "junto-coloquial-abrev",
			args:     args{resumeText: "meta de viagem, uns 400 mil"},
			expected: expected{goalPresent: true, valueCents: 40000000},
		},
		{
			name:     "junto-coloquial-milhao",
			args:     args{resumeText: "liberdade financeira, 1,5 milhão"},
			expected: expected{goalPresent: true, valueCents: 150000000},
		},
		{
			name:     "junto-mascara-medio",
			args:     args{resumeText: "quero trocar de carro, uns R$ 85.000,00"},
			expected: expected{goalPresent: true, valueCents: 8500000},
		},
		{
			name:     "junto-mil-reais-explicito",
			args:     args{resumeText: "juntar 50 mil reais pra dar entrada num apê"},
			expected: expected{goalPresent: true, valueCents: 5000000},
		},
		{
			name:     "junto-digitos-com-reais",
			args:     args{resumeText: "quitar o financiamento, faltam 120000 reais"},
			expected: expected{goalPresent: true, valueCents: 12000000},
		},
		{
			name:     "junto-milhoes-plural",
			args:     args{resumeText: "aposentadoria tranquila, 2 milhões"},
			expected: expected{goalPresent: true, valueCents: 200000000},
		},
		{
			name:     "junto-mil-pequeno",
			args:     args{resumeText: "montar um pé de meia de 5 mil"},
			expected: expected{goalPresent: true, valueCents: 500000},
		},
		{
			name:     "junto-mascara-notebook",
			args:     args{resumeText: "comprar um notebook, R$ 7.500,00"},
			expected: expected{goalPresent: true, valueCents: 750000},
		},
		{
			name:     "junto-mil-grande",
			args:     args{resumeText: "comprar um terreno, 250 mil"},
			expected: expected{goalPresent: true, valueCents: 25000000},
		},
		{
			name:     "junto-mascara-baixa",
			args:     args{resumeText: "trocar de celular, R$ 3.200,00"},
			expected: expected{goalPresent: true, valueCents: 320000},
		},
		{
			name:     "junto-mil-reais-intercambio",
			args:     args{resumeText: "fazer um intercâmbio, 30 mil reais"},
			expected: expected{goalPresent: true, valueCents: 3000000},
		},
		{
			name:     "so-meta-sem-valor",
			args:     args{resumeText: "quero quitar minhas dívidas", refusalText: "não sei"},
			expected: expected{goalPresent: true, valueCents: 0},
		},
		{
			name:     "so-meta-organizar",
			args:     args{resumeText: "quero organizar minha vida financeira", refusalText: "não faço ideia"},
			expected: expected{goalPresent: true, valueCents: 0},
		},
		{
			name:     "so-meta-habitos",
			args:     args{resumeText: "melhorar meus hábitos de consumo", refusalText: "prefiro não dizer"},
			expected: expected{goalPresent: true, valueCents: 0},
		},
		{
			name:     "meta-com-recusa-valor",
			args:     args{resumeText: "quero viajar, mas não sei quanto vou gastar", refusalText: "não quero informar agora"},
			expected: expected{goalPresent: true, valueCents: 0},
		},
		{
			name:     "valor-invalido-zero",
			args:     args{resumeText: "economizar, uns R$ 0", refusalText: "não sei quanto"},
			expected: expected{goalPresent: true, valueCents: 0},
		},
		{
			name:     "meta-recusa-seca",
			args:     args{resumeText: "guardar dinheiro", refusalText: "não"},
			expected: expected{goalPresent: true, valueCents: 0},
		},
	}

	hits := 0
	total := len(scenarios)
	step := workflows.BuildGoalStep(a)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			out, err := step(ctx, workflows.OnboardingState{
				UserID:     "11111111-1111-1111-1111-111111111111",
				ResumeText: scenario.args.resumeText,
			})

			ok := err == nil
			if ok && scenario.args.refusalText != "" {
				if out.Status != workflow.StepStatusSuspended || !out.State.GoalValueAsked {
					ok = false
				} else {
					resumeState := out.State
					resumeState.ResumeText = scenario.args.refusalText
					out, err = step(ctx, resumeState)
					ok = err == nil
				}
			}
			if ok && out.Status != workflow.StepStatusCompleted {
				ok = false
			}
			if ok {
				gotGoalPresent := out.State.Goal != ""
				if gotGoalPresent != scenario.expected.goalPresent || out.State.GoalValueCents != scenario.expected.valueCents {
					ok = false
				}
			}
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q status=%v goal=%q valueCents=%d esperado(goalPresent=%v,valueCents=%d) err=%v ok=%v",
				scenario.name, s.model, out.Status, out.State.Goal, out.State.GoalValueCents,
				scenario.expected.goalPresent, scenario.expected.valueCents, err, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM onboarding_goal_value modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-14/ADR-003: ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *OnboardingWorkflowRealLLMSuite) TestMethodologyParsesConfirmPercentReais() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, []tool.ToolHandle{}, nil, obs)

	const income int64 = 1350000

	type args struct {
		resumeText string
	}

	scenarios := []struct {
		name string
		args args
	}{
		{
			name: "confirmacao aceita a sugestao oficial",
			args: args{resumeText: "sim, pode confirmar"},
		},
		{
			name: "percentuais oficiais em porcentagem",
			args: args{resumeText: "Custo Fixo 40%, Conhecimento 10%, Prazeres 10%, Metas 10%, Liberdade Financeira 30%"},
		},
		{
			name: "valores em reais somando a renda",
			args: args{resumeText: "Custo Fixo R$ 5.400, Conhecimento R$ 1.350, Prazeres R$ 1.350, Metas R$ 1.350, Liberdade Financeira R$ 4.050"},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			budgets := func() *interfacemocks.BudgetPlanner {
				m := interfacemocks.NewBudgetPlanner(s.T())
				m.EXPECT().
					SuggestAllocation(mock.Anything, income, mock.Anything).
					Return(onboardingSuggestReturn(income), nil).
					Maybe()
				return m
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildMethodologyStep(a, budgets)
			out, err := step(ctx, workflows.OnboardingState{
				UserID:      "11111111-1111-1111-1111-111111111111",
				IncomeCents: income,
				ResumeText:  scenario.args.resumeText,
			})

			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusCompleted, out.Status)
			require.Equal(s.T(), 4000, out.State.Allocations["expense.custo_fixo"])
			require.Equal(s.T(), 1000, out.State.Allocations["expense.conhecimento"])
			require.Equal(s.T(), 1000, out.State.Allocations["expense.prazeres"])
			require.Equal(s.T(), 1000, out.State.Allocations["expense.metas"])
			require.Equal(s.T(), 3000, out.State.Allocations["expense.liberdade_financeira"])

			sum := 0
			for _, bp := range out.State.Allocations {
				sum += bp
			}
			require.Equal(s.T(), 10000, sum)
			s.T().Logf("caso %q → allocations %v", scenario.name, out.State.Allocations)
		})
	}
}
