//go:build integration

package e2e_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func realInterpreter(t *testing.T) usecases.IntentInterpreter {
	t.Helper()
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_real_budget"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)
	provider := openrouter.NewProvider(client, openrouter.ProviderConfig{
		Slug:        valueobjects.ModelSlugGeminiFlashLite(),
		APIKey:      os.Getenv("OPENROUTER_API_KEY"),
		HTTPReferer: "https://mecontrola.app",
		XTitle:      "MeControla",
		MaxTokens:   512,
		Temperature: 0,
	}, noop.NewProvider())
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{MaxFailures: 5, FailureWindow: 30 * time.Second, OpenDuration: 60 * time.Second})
	chain, err := services.NewFallbackChain([]appinterfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)
	return chain
}

func TestConfigureBudget_RealLLM_ExtractsAllocations(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	uc, err := usecases.NewConfigureBudgetConversation(realInterpreter(t), noop.NewProvider())
	require.NoError(t, err)

	out, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "quero um orçamento de 10 mil reais: custos fixos 35%, conhecimento 10%, prazeres 15%, metas 20%, liberdade financeira 20%",
		Draft: budgetdraft.Draft{},
	})
	require.NoError(t, err)
	require.True(t, out.Complete, "orçamento com renda + 5 percentuais somando 100%% deve ficar completo; reply=%q", out.Reply)
}

func TestConfigureBudget_RealLLM_MultiTurnAccumulates(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	uc, err := usecases.NewConfigureBudgetConversation(realInterpreter(t), noop.NewProvider())
	require.NoError(t, err)

	first, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "quero configurar um orçamento de 10 mil reais",
		Draft: budgetdraft.Draft{},
	})
	require.NoError(t, err)
	require.False(t, first.Complete, "só com a renda o orçamento não pode estar completo")

	second, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "custos fixos 35%, conhecimento 10%, prazeres 15%, metas 20% e liberdade financeira 20%",
		Draft: first.Draft,
	})
	require.NoError(t, err)
	require.True(t, second.Complete, "após renda + percentuais somando 100%% deve completar; reply=%q", second.Reply)
}
