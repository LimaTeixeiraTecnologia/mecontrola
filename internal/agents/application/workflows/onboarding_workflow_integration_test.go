//go:build integration

package workflows_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
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
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs, 0)

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

func (s *OnboardingWorkflowRealLLMSuite) TestBudgetReviewParsesConfirmPercentReais() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, []tool.ToolHandle{}, nil, obs, 0)

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
			name: "valores em reais somando o orcamento mensal",
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
				m.EXPECT().
					GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
					Return(interfaces.BudgetSummary{}, interfaces.ErrBudgetNotFound).
					Maybe()
				m.EXPECT().
					CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
					Return(interfaces.BudgetRef{}, nil).
					Maybe()
				return m
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildBudgetReviewStep(a, budgets)
			firstEntry, err := step(ctx, workflows.OnboardingState{
				UserID:             "11111111-1111-1111-1111-111111111111",
				MonthlyBudgetCents: income,
			})
			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, firstEntry.Status)

			resumeState := firstEntry.State
			resumeState.ResumeText = scenario.args.resumeText
			out, err := step(ctx, resumeState)

			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, out.Status)
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

func (s *OnboardingWorkflowRealLLMSuite) TestMonthlyBudgetExtractionGate() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs, 0)

	type args struct {
		resumeText string
	}
	type expected struct {
		cents int64
	}

	scenarios := []struct {
		name     string
		args     args
		expected expected
	}{
		{name: "mascarado-milhar", args: args{resumeText: "R$ 3.500,00"}, expected: expected{cents: 350000}},
		{name: "digitos-puros", args: args{resumeText: "3500"}, expected: expected{cents: 350000}},
		{name: "coloquial-mil", args: args{resumeText: "5 mil por mês"}, expected: expected{cents: 500000}},
		{name: "frase-completa", args: args{resumeText: "meu orçamento mensal é de R$ 4.200,00"}, expected: expected{cents: 420000}},
		{name: "valor-alto", args: args{resumeText: "uns 12 mil reais"}, expected: expected{cents: 1200000}},
		{name: "valor-baixo", args: args{resumeText: "800 reais"}, expected: expected{cents: 80000}},
		{name: "valor-medio-decimal", args: args{resumeText: "R$ 2.750,50"}, expected: expected{cents: 275050}},
		{name: "coloquial-mil-e-quebrado", args: args{resumeText: "6 mil e quinhentos"}, expected: expected{cents: 650000}},
		{name: "numero-redondo-grande", args: args{resumeText: "10000"}, expected: expected{cents: 1000000}},
		{name: "valor-pequeno-redondo", args: args{resumeText: "1200"}, expected: expected{cents: 120000}},
	}

	hits := 0
	total := len(scenarios)
	step := workflows.BuildMonthlyBudgetStep(a)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			out, err := step(ctx, workflows.OnboardingState{
				UserID:     "11111111-1111-1111-1111-111111111111",
				ResumeText: scenario.args.resumeText,
			})

			ok := err == nil && out.Status == workflow.StepStatusCompleted && out.State.MonthlyBudgetCents == scenario.expected.cents
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q status=%v cents=%d esperado=%d err=%v ok=%v",
				scenario.name, s.model, out.Status, out.State.MonthlyBudgetCents, scenario.expected.cents, err, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM onboarding_monthly_budget modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-42 (monthly_budget): ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *OnboardingWorkflowRealLLMSuite) TestSummaryConfirmExtractionGate() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs, 0)

	const income int64 = 500000

	type expected struct {
		confirmed bool
	}

	scenarios := []struct {
		name       string
		resumeText string
		expected   expected
	}{
		{name: "sim-simples", resumeText: "sim", expected: expected{confirmed: true}},
		{name: "sim-esta-correto", resumeText: "está correto, pode ativar", expected: expected{confirmed: true}},
		{name: "sim-confirmado", resumeText: "confirmado, pode seguir", expected: expected{confirmed: true}},
		{name: "nao-simples", resumeText: "não", expected: expected{confirmed: false}},
		{name: "nao-quero-revisar", resumeText: "não, quero revisar a distribuição", expected: expected{confirmed: false}},
		{name: "nao-mudar-valores", resumeText: "não, quero mudar os valores", expected: expected{confirmed: false}},
	}

	hits := 0
	total := len(scenarios)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			budgets := func() *interfacemocks.BudgetPlanner {
				m := interfacemocks.NewBudgetPlanner(s.T())
				m.EXPECT().
					SuggestAllocation(mock.Anything, income, mock.Anything).
					Return(onboardingSuggestReturn(income), nil).
					Maybe()
				m.EXPECT().
					GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
					Return(interfaces.BudgetSummary{}, interfaces.ErrBudgetNotFound).
					Maybe()
				m.EXPECT().
					CreateBudget(mock.Anything, mock.AnythingOfType("interfaces.DraftBudget")).
					Return(interfaces.BudgetRef{}, nil).
					Maybe()
				return m
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildBudgetReviewStep(a, budgets)
			firstEntry, err := step(ctx, workflows.OnboardingState{
				UserID:             "11111111-1111-1111-1111-111111111111",
				MonthlyBudgetCents: income,
			})
			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, firstEntry.Status)

			acceptState := firstEntry.State
			acceptState.ResumeText = "aceito a sugestão"
			confirmEntry, err := step(ctx, acceptState)
			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, confirmEntry.Status)

			resumeState := confirmEntry.State
			resumeState.ResumeText = scenario.resumeText
			out, err := step(ctx, resumeState)

			ok := err == nil
			if ok {
				if scenario.expected.confirmed {
					ok = out.Status == workflow.StepStatusCompleted
				} else {
					ok = out.Status == workflow.StepStatusSuspended && out.State.ReviewAwait.String() == "distribution"
				}
			}
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q status=%v reviewAwait=%v esperado(confirmed=%v) err=%v ok=%v",
				scenario.name, s.model, out.Status, out.State.ReviewAwait, scenario.expected.confirmed, err, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM onboarding_summary_confirm modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-42 (summary_confirm): ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *OnboardingWorkflowRealLLMSuite) TestRecurrenceExtractionGate() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs, 0)

	type expected struct {
		recurrence bool
	}

	scenarios := []struct {
		name       string
		resumeText string
		expected   expected
	}{
		{name: "sim-simples", resumeText: "sim", expected: expected{recurrence: true}},
		{name: "sim-quero-repetir", resumeText: "sim, quero repetir automaticamente", expected: expected{recurrence: true}},
		{name: "nao-simples", resumeText: "não", expected: expected{recurrence: false}},
		{name: "nao-prefiro-manual", resumeText: "não, prefiro configurar manualmente", expected: expected{recurrence: false}},
		{name: "ambiguo-talvez", resumeText: "talvez depois", expected: expected{recurrence: false}},
	}

	hits := 0
	total := len(scenarios)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			budgets := interfacemocks.NewBudgetPlanner(s.T())
			if scenario.expected.recurrence {
				budgets.EXPECT().
					CreateRecurrence(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), 12).
					Return(nil).
					Maybe()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildRecurrenceStep(a, budgets)
			firstEntry, err := step(ctx, workflows.OnboardingState{UserID: "11111111-1111-1111-1111-111111111111"})
			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, firstEntry.Status)

			resumeState := firstEntry.State
			resumeState.ResumeText = scenario.resumeText
			out, err := step(ctx, resumeState)

			ok := err == nil && out.Status == workflow.StepStatusCompleted && out.State.Recurrence == scenario.expected.recurrence
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q status=%v recurrence=%v esperado=%v err=%v ok=%v",
				scenario.name, s.model, out.Status, out.State.Recurrence, scenario.expected.recurrence, err, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM onboarding_recurrence modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-42 (recurrence): ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *OnboardingWorkflowRealLLMSuite) TestCardExtractionGate() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs, 0)

	type expected struct {
		wantsCard bool
	}

	scenarios := []struct {
		name       string
		resumeText string
		expected   expected
	}{
		{name: "completo-simples", resumeText: "Nubank, vencimento dia 10", expected: expected{wantsCard: true}},
		{name: "completo-frase", resumeText: "quero adicionar meu Itaú, vence todo dia 5", expected: expected{wantsCard: true}},
		{name: "completo-apelido-diferente", resumeText: "cartão do trabalho, banco Santander, dia 20", expected: expected{wantsCard: true}},
		{name: "recusa-nao", resumeText: "não", expected: expected{wantsCard: false}},
		{name: "recusa-nao-quero", resumeText: "não quero adicionar cartão", expected: expected{wantsCard: false}},
	}

	hits := 0
	total := len(scenarios)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			cards := interfacemocks.NewCardManager(s.T())
			cards.EXPECT().
				ListCards(mock.Anything, mock.AnythingOfType("uuid.UUID")).
				Return([]interfaces.Card{}, nil).
				Maybe()
			if scenario.expected.wantsCard {
				cards.EXPECT().
					CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
					Return(interfaces.CardRef{}, nil).
					Maybe()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildCardsStep(a, cards)
			firstEntry, err := step(ctx, workflows.OnboardingState{UserID: "11111111-1111-1111-1111-111111111111"})
			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, firstEntry.Status)

			resumeState := firstEntry.State
			resumeState.ResumeText = scenario.resumeText
			out, err := step(ctx, resumeState)

			ok := err == nil
			if ok {
				if scenario.expected.wantsCard {
					ok = out.Status == workflow.StepStatusSuspended && !out.State.CardsDone
				} else {
					ok = out.Status == workflow.StepStatusCompleted && out.State.CardsDone
				}
			}
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q status=%v cardsDone=%v esperado(wantsCard=%v) err=%v ok=%v",
				scenario.name, s.model, out.Status, out.State.CardsDone, scenario.expected.wantsCard, err, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM onboarding_card modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-42 (card): ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *OnboardingWorkflowRealLLMSuite) TestCardExtractionRealLLMGate() {
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs, 0)

	type expected struct {
		nickname string
		bank     string
		dueDay   int
	}

	scenarios := []struct {
		name       string
		resumeText string
		expected   expected
	}{
		{
			name:       "banco-sem-apelido-dia-primeiro",
			resumeText: "Nubank e vencimento dia primeiro",
			expected:   expected{nickname: "Nubank", bank: "Nubank", dueDay: 1},
		},
		{
			name:       "apelido-banco-dia-numerico",
			resumeText: "Roxinho, Nubank e vencimento dia 1",
			expected:   expected{nickname: "Roxinho", bank: "Nubank", dueDay: 1},
		},
		{
			name:       "banco-sem-apelido-dia-numerico",
			resumeText: "Nubank e vencimento dia 1",
			expected:   expected{nickname: "Nubank", bank: "Nubank", dueDay: 1},
		},
	}

	hits := 0
	total := len(scenarios)

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var captured interfaces.NewCard
			captureCard := false

			cards := interfacemocks.NewCardManager(s.T())
			cards.EXPECT().
				ListCards(mock.Anything, mock.AnythingOfType("uuid.UUID")).
				Return([]interfaces.Card{}, nil).
				Maybe()
			cards.EXPECT().
				CreateCard(mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
				RunAndReturn(func(_ context.Context, in interfaces.NewCard) (interfaces.CardRef, error) {
					captured = in
					captureCard = true
					return interfaces.CardRef{}, nil
				}).
				Maybe()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildCardsStep(a, cards)
			firstEntry, err := step(ctx, workflows.OnboardingState{UserID: "11111111-1111-1111-1111-111111111111"})
			require.NoError(s.T(), err)
			require.Equal(s.T(), workflow.StepStatusSuspended, firstEntry.Status)

			resumeState := firstEntry.State
			resumeState.ResumeText = scenario.resumeText
			out, err := step(ctx, resumeState)

			ok := err == nil
			if ok {
				ok = captureCard &&
					captured.Nickname == scenario.expected.nickname &&
					captured.Bank == scenario.expected.bank &&
					captured.DueDay == scenario.expected.dueDay
			}
			if ok {
				hits++
			}
			s.T().Logf("caso=%q modelo=%q status=%v captured=%+v esperado=%+v err=%v ok=%v",
				scenario.name, s.model, out.Status, captured, scenario.expected, err, ok)
		})
	}

	ratio := float64(hits) / float64(total)
	s.T().Logf("gate real-LLM onboarding_card_extraction modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(s.T(), ratio, 0.90, "gate de merge RF-07/RF-09 (card extraction): ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func TestCardFlow_Integration(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	userUUID := uuid.MustParse(userID)

	t.Run("banco unico preenche apelido e cria cartao valido", func(t *testing.T) {
		agentMock := agentmocks.NewAgent(t)
		payload, _ := json.Marshal(map[string]any{
			"wantsCard": true,
			"nickname":  "",
			"bank":      "Santander",
			"dueDay":    1,
		})
		agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: payload}, nil).Once()

		cardsMock := interfacemocks.NewCardManager(t)
		cardsMock.EXPECT().
			CreateCard(mock.Anything, interfaces.NewCard{UserID: userUUID, Nickname: "Santander", Bank: "Santander", DueDay: 1}).
			Return(interfaces.CardRef{}, nil).Once()
		cardsMock.EXPECT().
			ListCards(mock.Anything, userUUID).
			Return([]interfaces.Card{{Nickname: "Santander"}}, nil).Once()

		step := workflows.BuildCardsStep(agentMock, cardsMock)
		out, err := step(context.Background(), workflows.OnboardingState{
			UserID:     userID,
			Phase:      workflows.PhaseCards,
			ResumeText: "Santander, vencimento dia 1",
		})

		require.NoError(t, err)
		require.Equal(t, workflow.StepStatusSuspended, out.Status)
		require.NotNil(t, out.Suspend)
		require.Equal(t, "💳 Cartão registrado com sucesso ✅\nQuer registrar algum outro?", out.Suspend.Prompt)
		require.False(t, out.State.CardsDone)
	})

	t.Run("recusa sem cartao existente conclui etapa sem criar cartao", func(t *testing.T) {
		agentMock := agentmocks.NewAgent(t)
		payload, _ := json.Marshal(map[string]any{
			"wantsCard": false,
			"nickname":  "",
			"bank":      "",
			"dueDay":    0,
		})
		agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: payload}, nil).Once()

		cardsMock := interfacemocks.NewCardManager(t)

		step := workflows.BuildCardsStep(agentMock, cardsMock)
		out, err := step(context.Background(), workflows.OnboardingState{
			UserID:     userID,
			Phase:      workflows.PhaseCards,
			ResumeText: "não",
		})

		require.NoError(t, err)
		require.Equal(t, workflow.StepStatusCompleted, out.Status)
		require.True(t, out.State.CardsDone)
	})

	t.Run("cartao incompleto sem nome nao cria cartao e mantem workflow suspenso", func(t *testing.T) {
		agentMock := agentmocks.NewAgent(t)
		payload, _ := json.Marshal(map[string]any{
			"wantsCard": true,
			"nickname":  "",
			"bank":      "",
			"dueDay":    10,
		})
		agentMock.EXPECT().
			Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
			Return(agentpkg.Result{RawJSON: payload}, nil).Once()

		cardsMock := interfacemocks.NewCardManager(t)

		step := workflows.BuildCardsStep(agentMock, cardsMock)
		out, err := step(context.Background(), workflows.OnboardingState{
			UserID:     userID,
			Phase:      workflows.PhaseCards,
			ResumeText: "vencimento dia 10",
		})

		require.NoError(t, err)
		require.Equal(t, workflow.StepStatusSuspended, out.Status)
		require.NotNil(t, out.Suspend)
		require.NotContains(t, out.Suspend.Prompt, "💳")
		require.Contains(t, out.Suspend.Prompt, "cartão")
		require.False(t, out.State.CardsDone)
	})
}
