//go:build integration

package e2e_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func onboardingInterpreter(t *testing.T) usecases.IntentInterpreter {
	t.Helper()
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	model := os.Getenv("OPENROUTER_TEST_MODEL")
	if model == "" {
		model = "anthropic/claude-haiku-4.5"
	}
	slug, err := valueobjects.NewModelSlug(model)
	require.NoError(t, err)
	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_real_onboarding"),
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
	chain, err := services.NewFallbackChain([]interfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)
	return chain
}

type recordingReader struct {
	snapshot usecases.OnboardingSnapshot
}

func (r *recordingReader) Load(_ context.Context, _ uuid.UUID) (usecases.OnboardingSnapshot, error) {
	return r.snapshot, nil
}

type recordingDispatcher struct {
	calls []interfaces.ToolCall
}

func (d *recordingDispatcher) Dispatch(_ context.Context, _ uuid.UUID, _ string, call interfaces.ToolCall) (usecases.OnboardingToolResult, error) {
	d.calls = append(d.calls, call)
	return usecases.OnboardingToolResult{Reply: "ok", Advance: true}, nil
}

type recordingPhaseSetter struct{}

func (recordingPhaseSetter) SetPhase(_ context.Context, _ uuid.UUID, _ string) error { return nil }

func requireRealLLM(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")
}

func runRealOnboardingTurn(t *testing.T, snapshot usecases.OnboardingSnapshot, text string) (usecases.RunOnboardingTurnResult, *recordingDispatcher) {
	t.Helper()
	reader := &recordingReader{snapshot: snapshot}
	dispatcher := &recordingDispatcher{}
	uc, err := usecases.NewRunOnboardingTurn(onboardingInterpreter(t), reader, dispatcher, recordingPhaseSetter{}, 512, noop.NewProvider(), nil, noopV2Session{})
	require.NoError(t, err)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: text})
	require.NoError(t, err)
	require.True(t, out.Handled)
	return out, dispatcher
}

func TestOnboardingRealLLM_ObjectiveToolSelected(t *testing.T) {
	requireRealLLM(t)
	_, dispatcher := runRealOnboardingTurn(t, usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseObjective}, "quero fazer uma viagem")
	require.Len(t, dispatcher.calls, 1)
	require.Equal(t, usecases.ToolSaveOnboardingObjective, dispatcher.calls[0].FunctionName)
	require.NotEmpty(t, dispatcher.calls[0].ArgumentsJSON["objective"])
}

func TestOnboardingRealLLM_IncomeToolSelected(t *testing.T) {
	requireRealLLM(t)
	_, dispatcher := runRealOnboardingTurn(t, usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseBudget, Objective: "fazer uma viagem"}, "meu orçamento é 5 mil por mês")
	require.Len(t, dispatcher.calls, 1)
	require.Equal(t, usecases.ToolSaveOnboardingIncome, dispatcher.calls[0].FunctionName)
}

func TestOnboardingRealLLM_CardToolSelected(t *testing.T) {
	requireRealLLM(t)
	_, dispatcher := runRealOnboardingTurn(t, usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseCards, Objective: "viagem", IncomeCents: 500000}, "uso o nubank e vence dia 17")
	require.Len(t, dispatcher.calls, 1)
	require.Equal(t, usecases.ToolSaveOnboardingCard, dispatcher.calls[0].FunctionName)
}

func TestOnboardingRealLLM_BudgetSplitsToolSelected(t *testing.T) {
	requireRealLLM(t)
	_, dispatcher := runRealOnboardingTurn(t, usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseFinancialPlan, Objective: "viagem", IncomeCents: 500000}, "custo fixo 2000, conhecimento 500, prazeres 750, metas 1000 e liberdade financeira 750")
	require.Len(t, dispatcher.calls, 1)
	require.Equal(t, usecases.ToolSaveOnboardingBudgetSplits, dispatcher.calls[0].FunctionName)
}

func TestOnboardingRealLLM_QuestionStaysText(t *testing.T) {
	requireRealLLM(t)
	out, dispatcher := runRealOnboardingTurn(t, usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseBudget, Objective: "viagem"}, "por que vocês precisam saber meu orçamento?")
	require.Empty(t, dispatcher.calls)
	require.NotEmpty(t, out.Reply)
}
