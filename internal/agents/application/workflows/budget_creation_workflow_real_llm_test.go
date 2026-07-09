//go:build integration

package workflows_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type BudgetCreationExtractionRealLLMSuite struct {
	suite.Suite
	provider llm.Provider
	model    string
}

func TestBudgetCreationExtractionRealLLMSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationExtractionRealLLMSuite))
}

func (s *BudgetCreationExtractionRealLLMSuite) SetupSuite() {
	s.provider = buildRealLLMProvider(s.T())
	s.model = os.Getenv("AGENT_HARNESS_MODEL")
	if s.model == "" {
		s.model = "openai/gpt-4o-mini"
	}
}

func (s *BudgetCreationExtractionRealLLMSuite) TestBudgetTotalExtractionGate() {
	t := s.T()
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs)
	planner := imocks.NewBudgetPlanner(t)
	def := workflows.BuildBudgetCreationWorkflow(a, planner)

	type scenario struct {
		name          string
		totalText     string
		expectedCents int64
	}

	scenarios := []scenario{
		{name: "total-simples", totalText: "R$ 3.500,00", expectedCents: 350000},
		{name: "total-por-extenso", totalText: "três mil e quinhentos reais", expectedCents: 350000},
		{name: "total-mil-abreviado", totalText: "4 mil reais", expectedCents: 400000},
		{name: "total-decimal-simples", totalText: "2000", expectedCents: 200000},
		{name: "total-com-milhar-ponto", totalText: "R$ 1.200,50", expectedCents: 120050},
	}

	hits := 0
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			out, err := def.Root.Execute(ctx, workflows.BudgetCreationState{
				Awaiting:   workflows.AwaitingBudgetTotal,
				Competence: "2026-06",
				ResumeText: sc.totalText,
			})

			ok := err == nil
			if ok && out.State.TotalCents != sc.expectedCents {
				ok = false
			}
			if ok && out.State.Awaiting != workflows.AwaitingBudgetDistribution {
				ok = false
			}

			t.Logf("caso=%q modelo=%q totalCents=%d awaiting=%v esperado=%d err=%v ok=%v",
				sc.name, s.model, out.State.TotalCents, out.State.Awaiting, sc.expectedCents, err, ok)

			if ok {
				hits++
			}
		})
	}

	ratio := float64(hits) / float64(len(scenarios))
	t.Logf("gate real-LLM budget_total_extraction modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, len(scenarios), ratio)
	require.GreaterOrEqual(t, ratio, 0.90, "gate RF-04: ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *BudgetCreationExtractionRealLLMSuite) TestBudgetDistributionExtractionGate() {
	t := s.T()
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(s.provider, nil, nil, obs)
	planner := imocks.NewBudgetPlanner(t)
	def := workflows.BuildBudgetCreationWorkflow(a, planner)

	type scenario struct {
		name             string
		distributionText string
	}

	scenarios := []scenario{
		{name: "aceita-sugestao-sim", distributionText: "sim, aceito"},
		{name: "aceita-sugestao-pode", distributionText: "pode ser essa distribuição mesmo"},
		{name: "customiza-percentual", distributionText: "quero 50% custo fixo, 10% conhecimento, 10% prazeres, 10% metas e 20% liberdade financeira"},
	}

	hits := 0
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			out, err := def.Root.Execute(ctx, workflows.BudgetCreationState{
				Awaiting:   workflows.AwaitingBudgetDistribution,
				Competence: "2026-06",
				TotalCents: 350000,
				ResumeText: sc.distributionText,
			})

			ok := err == nil
			if ok && out.State.Awaiting != workflows.AwaitingBudgetConfirm {
				ok = false
			}
			if ok {
				sum := 0
				for _, bp := range out.State.Allocations {
					sum += bp
				}
				if sum != 10000 {
					ok = false
				}
			}

			t.Logf("caso=%q modelo=%q allocations=%v awaiting=%v err=%v ok=%v",
				sc.name, s.model, out.State.Allocations, out.State.Awaiting, err, ok)

			if ok {
				hits++
			}
		})
	}

	ratio := float64(hits) / float64(len(scenarios))
	t.Logf("gate real-LLM budget_distribution_extraction modelo=%q hits=%d total=%d ratio=%.4f", s.model, hits, len(scenarios), ratio)
	require.GreaterOrEqual(t, ratio, 0.90, "gate RF-04: ratio %.4f abaixo de 0.90 em %q", ratio, s.model)
}

func (s *BudgetCreationExtractionRealLLMSuite) TestBudgetConfirmationSimNaoGate() {
	t := s.T()

	allocations := map[string]int{
		"expense.custo_fixo":           4000,
		"expense.conhecimento":         1000,
		"expense.prazeres":             1000,
		"expense.metas":                1000,
		"expense.liberdade_financeira": 3000,
	}
	userID := uuid.MustParse("00000000-0000-0000-0000-0000000000e2")

	type scenario struct {
		name         string
		confirmText  string
		expectStatus workflows.BudgetCreationStatus
	}

	scenarios := []scenario{
		{name: "confirma-sim", confirmText: "sim", expectStatus: workflows.BudgetCreationCompleted},
		{name: "confirma-confirma", confirmText: "confirma", expectStatus: workflows.BudgetCreationCompleted},
		{name: "confirma-pode", confirmText: "pode", expectStatus: workflows.BudgetCreationCompleted},
		{name: "nega-nao", confirmText: "não", expectStatus: workflows.BudgetCreationCancelled},
		{name: "nega-cancela", confirmText: "cancela", expectStatus: workflows.BudgetCreationCancelled},
	}

	hits := 0
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			planner := imocks.NewBudgetPlanner(t)
			if sc.expectStatus == workflows.BudgetCreationCompleted {
				planner.EXPECT().CreateBudget(context.Background(), agentsifaces.DraftBudget{
					UserID:     userID,
					Competence: "2026-06",
					TotalCents: 350000,
					Allocations: []agentsifaces.AllocationDraft{
						{RootSlug: "expense.custo_fixo", BasisPoints: 4000},
						{RootSlug: "expense.conhecimento", BasisPoints: 1000},
						{RootSlug: "expense.prazeres", BasisPoints: 1000},
						{RootSlug: "expense.metas", BasisPoints: 1000},
						{RootSlug: "expense.liberdade_financeira", BasisPoints: 3000},
					},
				}).Return(agentsifaces.BudgetRef{}, nil).Maybe()
				planner.EXPECT().ActivateBudget(context.Background(), userID, "2026-06").
					Return(nil).Maybe()
			}
			def := workflows.BuildBudgetCreationWorkflow(nil, planner)

			out, err := def.Root.Execute(context.Background(), workflows.BudgetCreationState{
				Awaiting:    workflows.AwaitingBudgetConfirm,
				Competence:  "2026-06",
				UserID:      userID,
				TotalCents:  350000,
				Allocations: allocations,
				ResumeText:  sc.confirmText,
			})

			ok := err == nil
			if ok && out.Status != workflow.StepStatusCompleted {
				ok = false
			}
			if ok && out.State.Status != sc.expectStatus {
				ok = false
			}

			t.Logf("caso=%q status=%v err=%v ok=%v", sc.name, out.State.Status, err, ok)
			if ok {
				hits++
			}
		})
	}

	ratio := float64(hits) / float64(len(scenarios))
	t.Logf("gate confirmacao determinística hits=%d total=%d ratio=%.4f", hits, len(scenarios), ratio)
	require.GreaterOrEqual(t, ratio, 0.90, "gate RF-08/RF-09: ratio %.4f abaixo de 0.90", ratio)
}
