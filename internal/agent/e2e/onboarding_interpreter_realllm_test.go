//go:build integration

package e2e_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func realOnboardingInterpreter(t *testing.T) agentwf.OnboardingInterpreter {
	t.Helper()
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	model := os.Getenv("LLM_TEST_MODEL")
	if model == "" {
		model = "google/gemini-2.5-flash-lite"
	}
	slug, err := valueobjects.NewModelSlug(model)
	require.NoError(t, err)
	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_onboarding"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)
	provider := openrouter.NewProvider(client, openrouter.ProviderConfig{
		Slug:        slug,
		APIKey:      os.Getenv("OPENROUTER_API_KEY"),
		HTTPReferer: "https://mecontrola.app",
		XTitle:      "MeControla",
		MaxTokens:   512,
		Temperature: 0,
	}, noop.NewProvider())
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{MaxFailures: 5, FailureWindow: 30 * time.Second, OpenDuration: 60 * time.Second})
	chain, err := services.NewFallbackChain([]appinterfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)
	return onboarding.NewOnboardingInterpreter(chain, 512)
}

func TestOnboardingInterpreter_RealLLM_ParsesInputs(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	interp := realOnboardingInterpreter(t)
	ctx := context.Background()

	t.Run("objective natural language", func(t *testing.T) {
		obj, err := interp.ParseObjective(ctx, "tenho umas dívidas no cartão e queria me livrar delas esse ano")
		require.NoError(t, err)
		require.False(t, obj.DailyCommand)
		require.False(t, obj.Ambiguous)
		require.NotEmpty(t, obj.Objective)
		t.Logf("objective => %q", obj.Objective)
	})

	t.Run("objective off-topic clarifies", func(t *testing.T) {
		obj, err := interp.ParseObjective(ctx, "qual a capital da França?")
		require.NoError(t, err)
		require.True(t, obj.Ambiguous || obj.DailyCommand, "off-topic deve clarify/daily, veio objective=%q", obj.Objective)
	})

	t.Run("budget with currency", func(t *testing.T) {
		b, err := interp.ParseBudget(ctx, "recebo uns 4 mil por mês")
		require.NoError(t, err)
		require.False(t, b.Ambiguous)
		require.False(t, b.DailyCommand)
		require.Equal(t, int64(400000), b.IncomeCents)
	})

	t.Run("budget no value clarifies", func(t *testing.T) {
		b, err := interp.ParseBudget(ctx, "não sei dizer agora")
		require.NoError(t, err)
		require.True(t, b.Ambiguous)
	})

	t.Run("card nickname and due day", func(t *testing.T) {
		c, err := interp.ParseCards(ctx, "tenho o Nubank que vence dia 13", 0)
		require.NoError(t, err)
		require.Equal(t, "Nubank", c.Nickname)
		require.Equal(t, 13, c.DueDay)
	})

	t.Run("card skip", func(t *testing.T) {
		c, err := interp.ParseCards(ctx, "não uso cartão de crédito", 0)
		require.NoError(t, err)
		require.True(t, c.Skip)
	})

	t.Run("categories confirm flexible wording", func(t *testing.T) {
		ok, err := interp.ParseCategoriesConfirm(ctx, "claro, faz total sentido")
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("categories question clarifies", func(t *testing.T) {
		ok, err := interp.ParseCategoriesConfirm(ctx, "o que é liberdade financeira?")
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("value monetary", func(t *testing.T) {
		v, err := interp.ParseValue(ctx, "uns mil e quinhentos reais")
		require.NoError(t, err)
		require.False(t, v.Ambiguous)
		require.Equal(t, int64(150000), v.ValueCents)
	})
}
