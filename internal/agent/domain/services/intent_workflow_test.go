package services_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func mustIntent(t *testing.T, module valueobjects.IntentModule, action valueobjects.IntentAction, hint string) entities.IntentResult {
	t.Helper()
	r, err := entities.NewIntentResult(module, action, json.RawMessage(`{}`), json.RawMessage(`{}`), hint)
	require.NoError(t, err)
	return r
}

func TestIntentWorkflow_DecideRoute(t *testing.T) {
	sut := services.NewIntentWorkflow()
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	eventID := uuid.New()
	provider := valueobjects.ModelSlugGeminiFlashLite()

	t.Run("routed for supported module+action", func(t *testing.T) {
		intent := mustIntent(t, valueobjects.IntentModuleTransactions(), valueobjects.IntentActionCreate(), "ok")
		outcome := sut.DecideRoute(intent, provider, eventID, now)
		assert.Equal(t, services.IntentOutcomeRouted, outcome.Kind)
		assert.Equal(t, eventID, outcome.EventID)
		assert.True(t, outcome.Provider.Equal(provider))
	})

	t.Run("structured error pass-through", func(t *testing.T) {
		intent := entities.NewIntentResultFromError(entities.IntentError{Code: "not_found", Message: "x"})
		outcome := sut.DecideRoute(intent, provider, eventID, now)
		assert.Equal(t, services.IntentOutcomeStructuredError, outcome.Kind)
		assert.Equal(t, "not_found", outcome.Reason)
	})

	t.Run("unsupported action for module", func(t *testing.T) {
		intent := mustIntent(t, valueobjects.IntentModuleCategories(), valueobjects.IntentActionCreate(), "ok")
		outcome := sut.DecideRoute(intent, provider, eventID, now)
		assert.Equal(t, services.IntentOutcomeUnsupportedAction, outcome.Kind)
		assert.Equal(t, "action_not_supported_for_module", outcome.Reason)
	})
}

func TestIntentWorkflow_DecideExhausted(t *testing.T) {
	sut := services.NewIntentWorkflow()
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	eventID := uuid.New()

	outcome := sut.DecideExhausted("upstream_5xx", eventID, now)
	assert.Equal(t, services.IntentOutcomeProviderExhausted, outcome.Kind)
	assert.Equal(t, "upstream_5xx", outcome.Reason)
	assert.NotEmpty(t, outcome.ResponseHint)
}

func TestIntentOutcomeKind_String(t *testing.T) {
	cases := map[services.IntentOutcomeKind]string{
		services.IntentOutcomeRouted:            "routed",
		services.IntentOutcomeStructuredError:   "structured_error",
		services.IntentOutcomeProviderExhausted: "provider_exhausted",
		services.IntentOutcomeUnsupportedAction: "unsupported_action",
		services.IntentOutcomeKind(0):           "invalid",
		services.IntentOutcomeKind(99):          "invalid",
	}
	for k, want := range cases {
		assert.Equal(t, want, k.String())
	}
}
