//go:build integration

package e2e_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func budgetRealParser(t *testing.T) *usecases.ParseInbound {
	t.Helper()
	slug, err := valueobjects.NewModelSlug("google/gemini-2.5-flash-lite")
	require.NoError(t, err)
	return realParserForModel(t, slug)
}

func parsedBudgetChange(out usecases.ParseInboundOutput) budgetdraft.Change {
	return budgetdraft.Change{
		TotalCents:  out.Intent.BudgetTotalCents(),
		Allocations: out.Intent.BudgetAllocations(),
	}
}

func TestConfigureBudget_RealLLM_ExtractsAllocations(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	parser := budgetRealParser(t)

	var complete bool
	for attempt := 0; attempt < 3; attempt++ {
		out, execErr := parser.Execute(context.Background(), usecases.ParseInboundInput{
			UserID: uuid.New(),
			Text:   "quero um orçamento de 10 mil reais: custos fixos 35%, conhecimento 10%, prazeres 15%, metas 20%, liberdade financeira 20%",
		})
		require.NoError(t, execErr)
		require.Equal(t, intent.KindConfigureBudget, out.Intent.Kind(), "parse deve classificar como configure_budget")

		merged, mergeErr := budgetdraft.New("2026-06").Merge(parsedBudgetChange(out))
		require.NoError(t, mergeErr)
		if merged.IsComplete() {
			complete = true
			break
		}
	}
	require.True(t, complete, "parse com renda + 5 percentuais somando 100%% deve extrair alocações completas em 3 tentativas")
}

func TestConfigureBudget_RealLLM_MultiTurnAccumulates(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	parser := budgetRealParser(t)

	var complete bool
	for attempt := 0; attempt < 3; attempt++ {
		firstOut, execErr := parser.Execute(context.Background(), usecases.ParseInboundInput{
			UserID: uuid.New(),
			Text:   "quero configurar um orçamento de 10 mil reais",
		})
		require.NoError(t, execErr)
		require.Equal(t, intent.KindConfigureBudget, firstOut.Intent.Kind())

		draft, mergeErr := budgetdraft.New("2026-06").Merge(parsedBudgetChange(firstOut))
		require.NoError(t, mergeErr)
		require.False(t, draft.IsComplete(), "só com a renda o orçamento não pode estar completo")

		secondOut, execErr := parser.Execute(context.Background(), usecases.ParseInboundInput{
			UserID: uuid.New(),
			Text:   "custos fixos 35%, conhecimento 10%, prazeres 15%, metas 20% e liberdade financeira 20%",
		})
		require.NoError(t, execErr)
		require.Equal(t, intent.KindConfigureBudget, secondOut.Intent.Kind())

		merged, mergeErr := draft.Merge(parsedBudgetChange(secondOut))
		require.NoError(t, mergeErr)
		if merged.IsComplete() {
			complete = true
			break
		}
	}
	require.True(t, complete, "após renda + percentuais somando 100%% o merge determinístico deve completar em 3 tentativas")
}
