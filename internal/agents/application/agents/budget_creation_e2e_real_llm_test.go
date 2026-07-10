//go:build integration

package agents

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeBudgetE2EEngine struct {
	startResult workflow.RunResult[workflows.BudgetCreationState]
	startErr    error
	startCalled bool
	lastState   workflows.BudgetCreationState
}

func (f *fakeBudgetE2EEngine) Start(_ context.Context, _ workflow.Definition[workflows.BudgetCreationState], _ string, initial workflows.BudgetCreationState) (workflow.RunResult[workflows.BudgetCreationState], error) {
	f.startCalled = true
	f.lastState = initial
	return f.startResult, f.startErr
}

func (f *fakeBudgetE2EEngine) Resume(_ context.Context, _ workflow.Definition[workflows.BudgetCreationState], _ string, _ []byte) (workflow.RunResult[workflows.BudgetCreationState], error) {
	return workflow.RunResult[workflows.BudgetCreationState]{}, nil
}

func (f *fakeBudgetE2EEngine) LoadLatestState(_ context.Context, _ workflow.Definition[workflows.BudgetCreationState], _ string) (workflows.BudgetCreationState, workflow.Snapshot, bool, error) {
	return workflows.BudgetCreationState{}, workflow.Snapshot{}, false, nil
}

func newSuspendedBudgetE2EEngine(prompt string) *fakeBudgetE2EEngine {
	return &fakeBudgetE2EEngine{
		startResult: workflow.RunResult[workflows.BudgetCreationState]{
			Status: workflow.RunStatusSuspended,
			State:  workflows.BudgetCreationState{ResponseText: prompt},
		},
	}
}

func fakeBudgetE2EDef() workflow.Definition[workflows.BudgetCreationState] {
	return workflow.Definition[workflows.BudgetCreationState]{
		ID:      workflows.BudgetCreationWorkflowID,
		Durable: true,
	}
}

func budgetE2EInboundCtx(userID uuid.UUID, message, messageID string) context.Context {
	req := agent.InboundRequest{
		ResourceID: userID.String(),
		ThreadID:   "thread-e2e",
		AgentID:    MecontrolaAgentID,
		Message:    message,
		MessageID:  messageID,
	}
	ctx := workflow.WithRuntime(context.Background(), req)
	return agent.WithToolInvocationContext(ctx, userID.String(), messageID, 0)
}

func splitCompetence(competence string) (year, month int) {
	if len(competence) != 7 || competence[4] != '-' {
		return 0, 0
	}
	y, errY := strconv.Atoi(competence[0:4])
	m, errM := strconv.Atoi(competence[5:7])
	if errY != nil || errM != nil {
		return 0, 0
	}
	return y, m
}

type BudgetCreationE2ERealLLMSuite struct {
	suite.Suite
	provider llm.Provider
	model    string
}

func TestBudgetCreationE2ERealLLMSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationE2ERealLLMSuite))
}

func (s *BudgetCreationE2ERealLLMSuite) SetupSuite() {
	s.provider = buildRealLLMProvider(s.T())
	s.model = os.Getenv("AGENT_HARNESS_MODEL")
	if s.model == "" {
		s.model = "openai/gpt-4o-mini"
	}
}

func (s *BudgetCreationE2ERealLLMSuite) TestCreateBudgetRoutingGate() {
	t := s.T()

	type scenario struct {
		name          string
		message       string
		expectStarted bool
		expectYear    int
		expectMonth   int
		checkResponse func(t *testing.T, reply string) bool
	}

	scenarios := []scenario{
		{
			name:          "criacao com distribuicao mes atual",
			message:       "quero criar um orçamento para esse mês com total de R$ 3.500,00",
			expectStarted: true,
		},
		{
			name:          "retroativo junho 2026",
			message:       "cria um orçamento pra junho de 2026",
			expectStarted: true,
			expectYear:    2026,
			expectMonth:   6,
		},
		{
			name:          "antigo janeiro 2025",
			message:       "quero criar o orçamento de janeiro de 2025",
			expectStarted: true,
			expectYear:    2025,
			expectMonth:   1,
		},
		{
			name:          "mes passado por extenso",
			message:       "cria o orçamento do mês passado",
			expectStarted: true,
		},
		{
			name:          "mes sem ano clarifica",
			message:       "quero criar o orçamento de junho",
			expectStarted: false,
			checkResponse: func(t *testing.T, reply string) bool {
				lower := strings.ToLower(reply)
				return strings.Contains(lower, "ano")
			},
		},
	}

	const repeats = 3

	hits := 0
	total := 0
	for _, sc := range scenarios {
		for r := 1; r <= repeats; r++ {
			total++
			s.Run(fmt.Sprintf("%s/%d", sc.name, r), func() {
				userID := uuid.New()
				obs := fake.NewProvider()

				engine := newSuspendedBudgetE2EEngine("Vamos criar seu orçamento. Qual é o valor total?")
				def := fakeBudgetE2EDef()

				createBudgetTool := agenttools.BuildCreateBudgetTool(engine, def)
				tools := []tool.ToolHandle{createBudgetTool}

				a := BuildMeControlaAgent(s.provider, tools, nil, obs, 0)
				ctx := budgetE2EInboundCtx(userID, sc.message, "wamid-route-"+uuid.NewString())
				ctx, cancel := context.WithTimeout(ctx, 90*time.Second)

				result, err := a.Execute(ctx, agent.Request{
					AgentID: MecontrolaAgentID,
					Messages: []llm.Message{
						{Role: "user", Content: sc.message},
					},
					MaxTokens: 1024,
				})
				cancel()
				require.NoError(t, err)

				ok := true
				if sc.expectStarted {
					if !engine.startCalled {
						ok = false
						t.Logf("%s: create_budget não foi chamado: %s", sc.name, result.Content)
					} else if sc.expectYear != 0 {
						gotYear, gotMonth := splitCompetence(engine.lastState.Competence)
						if gotYear != sc.expectYear || gotMonth != sc.expectMonth {
							ok = false
							t.Logf("%s: competence esperado=%d-%02d obtido=%s", sc.name, sc.expectYear, sc.expectMonth, engine.lastState.Competence)
						}
					}
				} else {
					if engine.startCalled {
						ok = false
						t.Logf("%s: create_budget não deveria iniciar workflow: %s (competence=%s)", sc.name, result.Content, engine.lastState.Competence)
					}
					if ok && sc.checkResponse != nil && !sc.checkResponse(t, result.Content) {
						ok = false
						t.Logf("%s: checkResponse falhou, resposta=%q", sc.name, result.Content)
					}
				}

				if ok {
					hits++
				}
			})
		}
	}

	ratio := float64(hits) / float64(total)
	t.Logf("gate real-LLM create_budget routing modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(t, ratio, 0.90, "gate RF-04/RF-05/RF-08/RF-09/RF-13..RF-16: ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *BudgetCreationE2ERealLLMSuite) TestRetrospectivaCompositionGate() {
	t := s.T()

	type scenario struct {
		name        string
		message     string
		planOutcome string
		planResult  agentsifaces.BudgetSummary
		containsAny []string
	}

	plannedTotal := int64(500000)
	spentTotal := int64(200000)

	scenarios := []scenario{
		{
			name:        "retrospectiva com orcamento",
			message:     "como foi meu mês de junho de 2026?",
			planOutcome: "ok",
			planResult: agentsifaces.BudgetSummary{
				Competence: "2026-06",
				TotalCents: &plannedTotal,
				State:      "active",
				Allocations: []agentsifaces.AllocationSummary{
					{RootSlug: "expense.custo_fixo", PlannedCents: &plannedTotal, SpentCents: spentTotal},
				},
				TotalSpentCents: spentTotal,
			},
			containsAny: []string{"junho de 2026", "planejad", "realizad"},
		},
		{
			name:        "retrospectiva sem orcamento com lancamentos",
			message:     "como foi meu mês de maio de 2026?",
			planOutcome: "not_found",
			containsAny: []string{"ajudar a criar", "criar um orçamento", "criar seu orçamento", "posso te ajudar"},
		},
	}

	const repeats = 3

	hits := 0
	total := 0
	for _, sc := range scenarios {
		for r := 1; r <= repeats; r++ {
			total++
			s.Run(fmt.Sprintf("%s/%d", sc.name, r), func() {
				userID := uuid.New()
				obs := fake.NewProvider()

				plannerMock := imocks.NewBudgetPlanner(t)
				ledgerMock := imocks.NewTransactionsLedger(t)

				if sc.planOutcome == "ok" {
					plannerMock.EXPECT().GetMonthlySummary(mock.Anything, mock.Anything, mock.Anything).
						Return(sc.planResult, nil).Maybe()
					plannerMock.EXPECT().ListAlerts(mock.Anything, mock.Anything).
						Return(nil, nil).Maybe()
				} else {
					plannerMock.EXPECT().GetMonthlySummary(mock.Anything, mock.Anything, mock.Anything).
						Return(agentsifaces.BudgetSummary{}, agentsifaces.ErrBudgetNotFound).Maybe()
				}
				ledgerMock.EXPECT().GetMonthlySummary(mock.Anything, mock.Anything, mock.Anything).
					Return(agentsifaces.MonthlySummary{RefMonth: "2026-05", IncomeCents: 0, OutcomeCents: 150000, TotalCents: 150000}, nil).Maybe()
				ledgerMock.EXPECT().ListMonthlyEntries(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return([]agentsifaces.MonthlyEntry{{ID: uuid.NewString(), RefMonth: "2026-05", AmountCents: 150000, Direction: "outcome"}}, nil).Maybe()

				tools := []tool.ToolHandle{
					agenttools.BuildQueryPlanTool(plannerMock),
					agenttools.BuildQueryMonthTool(ledgerMock),
				}

				a := BuildMeControlaAgent(s.provider, tools, nil, obs, 0)
				ctx := budgetE2EInboundCtx(userID, sc.message, "wamid-retro-"+uuid.NewString())
				ctx, cancel := context.WithTimeout(ctx, 90*time.Second)

				result, err := a.Execute(ctx, agent.Request{
					AgentID: MecontrolaAgentID,
					Messages: []llm.Message{
						{Role: "user", Content: sc.message},
					},
					MaxTokens: 1024,
				})
				cancel()
				require.NoError(t, err)

				lower := strings.ToLower(result.Content)
				ok := false
				for _, term := range sc.containsAny {
					if strings.Contains(lower, strings.ToLower(term)) {
						ok = true
						break
					}
				}
				t.Logf("caso=%q modelo=%q resposta=%q ok=%v toolCalls=%+v", sc.name, s.model, result.Content, ok, result.ToolCalls)
				if ok {
					hits++
				}
			})
		}
	}

	ratio := float64(hits) / float64(total)
	t.Logf("gate real-LLM retrospectiva modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, total, ratio)
	require.GreaterOrEqual(t, ratio, 0.90, "gate RF-22/RF-23/RF-24: ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *BudgetCreationE2ERealLLMSuite) TestFailurePersistenceMessageSpecific() {
	t := s.T()
	obs := fake.NewProvider()

	engine := newSuspendedBudgetE2EEngine("Vamos criar seu orçamento. Qual é o valor total?")
	def := fakeBudgetE2EDef()
	tools := []tool.ToolHandle{agenttools.BuildCreateBudgetTool(engine, def)}

	a := BuildMeControlaAgent(s.provider, tools, nil, obs, 0)
	userID := uuid.New()
	ctx := budgetE2EInboundCtx(userID, "quero criar um orçamento para este mês", "wamid-fail-"+uuid.NewString())
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "quero criar um orçamento para este mês"},
		},
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)

	lower := strings.ToLower(result.Content)
	require.NotContains(t, lower, "não entendi", "RF-26: mensagem de falha de execução deve ser distinta do fallback de não entendi")
}
