//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func guardInterpreter(t *testing.T, model string) *services.FallbackChain {
	t.Helper()
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	slug, err := valueobjects.NewModelSlug(model)
	require.NoError(t, err)
	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_guard"),
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
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{
		MaxFailures:   5,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	})
	chain, err := services.NewFallbackChain([]interfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)
	return chain
}

func TestStructuredOutputGuard_ParseClass_StrictTrue(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	eligibleModels := []string{
		"google/gemini-2.5-flash-lite",
		"mistralai/mistral-small-3.2-24b-instruct",
	}

	schema := &interfaces.JSONSchemaSpec{
		Name:   "mecontrola_parse_intent",
		Strict: true,
		Schema: prompting.ParseIntentJSONSchema(),
	}

	for _, model := range eligibleModels {
		model := model
		t.Run(model, func(t *testing.T) {
			interp := guardInterpreter(t, model)
			system, err := prompting.RenderSystem()
			require.NoError(t, err)
			user, err := prompting.RenderUser("gastei 58 no ifood")
			require.NoError(t, err)

			resp, err := interp.Interpret(context.Background(), interfaces.LLMRequest{
				SystemPrompt: system,
				UserMessage:  user,
				JSONSchema:   schema,
			})
			require.NoError(t, err, "modelo %s deve responder sem erro com Strict=true", model)
			require.NotEmpty(t, resp.RawJSON, "modelo %s deve retornar JSON nao vazio", model)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal(resp.RawJSON, &parsed), "modelo %s deve retornar JSON valido", model)
			require.Contains(t, parsed, "kind", "modelo %s deve incluir campo 'kind' no JSON", model)
		})
	}
}

func TestStructuredOutputGuard_OnboardingClass_StrictTrue(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	model := os.Getenv("OPENROUTER_TEST_MODEL")
	if model == "" {
		model = "google/gemini-2.5-flash-lite"
	}

	schema := &interfaces.JSONSchemaSpec{
		Name:   "onboarding_objective",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":            map[string]any{"type": "string", "enum": []string{"save_onboarding_objective", "clarify"}},
				"objective":         map[string]any{"type": "string", "maxLength": 280},
				"objective_profile": map[string]any{"type": "string", "enum": []string{"", "payoff_debt", "emergency_fund", "invest", "specific_goal", "organize_spending"}},
				"reply":             map[string]any{"type": "string", "maxLength": 1000},
			},
			"required":             []string{"action", "objective", "objective_profile", "reply"},
			"additionalProperties": false,
		},
	}

	interp := guardInterpreter(t, model)
	resp, err := interp.Interpret(context.Background(), interfaces.LLMRequest{
		SystemPrompt: "Voce e o assistente financeiro do MeControla. Identifique o objetivo financeiro do usuario.",
		UserMessage:  "Quero quitar minhas dividas",
		JSONSchema:   schema,
	})
	require.NoError(t, err, "modelo %s deve responder sem erro com Strict=true no onboarding", model)
	require.NotEmpty(t, resp.RawJSON, "modelo %s deve retornar JSON nao vazio", model)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(resp.RawJSON, &parsed), "modelo %s deve retornar JSON valido", model)
	require.Contains(t, parsed, "action", "resposta deve conter campo 'action'")
}
