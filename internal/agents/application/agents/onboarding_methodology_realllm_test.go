//go:build integration

package agents

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

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

func TestRealLLM_OnboardingMethodology_ParsesConfirmPercentReais(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()
	a := BuildMeControlaAgent(provider, []tool.ToolHandle{}, nil, obs)

	const income int64 = 1350000

	scenarios := []struct {
		name       string
		resumeText string
	}{
		{
			name:       "confirmacao aceita a sugestao oficial",
			resumeText: "sim, pode confirmar",
		},
		{
			name:       "percentuais oficiais em porcentagem",
			resumeText: "Custo Fixo 40%, Conhecimento 10%, Prazeres 10%, Metas 10%, Liberdade Financeira 30%",
		},
		{
			name:       "valores em reais somando a renda",
			resumeText: "Custo Fixo R$ 5.400, Conhecimento R$ 1.350, Prazeres R$ 1.350, Metas R$ 1.350, Liberdade Financeira R$ 4.050",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			budgets := interfacemocks.NewBudgetPlanner(t)
			budgets.EXPECT().
				SuggestAllocation(mock.Anything, income, mock.Anything).
				Return(onboardingSuggestReturn(income), nil).
				Maybe()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			step := workflows.BuildMethodologyStep(a, budgets)
			out, err := step(ctx, workflows.OnboardingState{
				UserID:      "11111111-1111-1111-1111-111111111111",
				IncomeCents: income,
				ResumeText:  scenario.resumeText,
			})

			require.NoError(t, err)
			require.Equal(t, workflow.StepStatusCompleted, out.Status)
			require.Equal(t, 4000, out.State.Allocations["expense.custo_fixo"])
			require.Equal(t, 1000, out.State.Allocations["expense.conhecimento"])
			require.Equal(t, 1000, out.State.Allocations["expense.prazeres"])
			require.Equal(t, 1000, out.State.Allocations["expense.metas"])
			require.Equal(t, 3000, out.State.Allocations["expense.liberdade_financeira"])

			sum := 0
			for _, bp := range out.State.Allocations {
				sum += bp
			}
			require.Equal(t, 10000, sum)
			t.Logf("caso %q → allocations %v", scenario.name, out.State.Allocations)
		})
	}
}
