package services_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func TestIntentValidator_HappyPath(t *testing.T) {
	sut := services.NewIntentValidator()
	raw := []byte(`{
		"module":"transactions",
		"action":"create",
		"payload":{"amount":50,"type":"expense","description":"almoco","occurred_at":"2026-06-13","category_id":"3b3b3b3b-0000-0000-0000-000000000001"},
		"response_hint":"Anotei seu gasto de R$ 50 no almoco."
	}`)

	intent, err := sut.Validate(raw)
	require.NoError(t, err)
	assert.False(t, intent.IsError())
	assert.True(t, intent.Module().Equal(valueobjects.IntentModuleTransactions()))
	assert.True(t, intent.Action().Equal(valueobjects.IntentActionCreate()))
	assert.Contains(t, intent.ResponseHint(), "almoco")
}

func TestIntentValidator_StructuredError(t *testing.T) {
	sut := services.NewIntentValidator()
	raw := []byte(`{"error":"out_of_scope","message":"So consigo ajudar com financas."}`)

	intent, err := sut.Validate(raw)
	require.NoError(t, err)
	assert.True(t, intent.IsError())
	assert.Equal(t, "out_of_scope", intent.Error().Code)
}

func TestIntentValidator_RejectsForbiddenKeys(t *testing.T) {
	sut := services.NewIntentValidator()
	cases := []struct {
		name string
		raw  string
	}{
		{name: "user_id in payload", raw: `{"module":"cards","action":"list","payload":{"user_id":"injected"},"response_hint":"ok"}`},
		{name: "tenant_id in payload", raw: `{"module":"cards","action":"list","payload":{"tenant_id":"x"},"response_hint":"ok"}`},
		{name: "userId camelCase in filters", raw: `{"module":"cards","action":"list","filters":{"userId":"x"},"response_hint":"ok"}`},
		{name: "user-id kebab in filters", raw: `{"module":"cards","action":"list","filters":{"user-id":"x"},"response_hint":"ok"}`},
		{name: "principal_id nested", raw: `{"module":"transactions","action":"list","filters":{"category":{"principal_id":"x"}},"response_hint":"ok"}`},
		{name: "user_id nested in array", raw: `{"module":"transactions","action":"create","payload":{"items":[{"user_id":"x"}]},"response_hint":"ok"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sut.Validate([]byte(tc.raw))
			require.Error(t, err)
			assert.True(t, errors.Is(err, services.ErrValidatorForbiddenField), "got %v", err)
		})
	}
}

func TestIntentValidator_RejectsInvalidModule(t *testing.T) {
	sut := services.NewIntentValidator()
	_, err := sut.Validate([]byte(`{"module":"accounts","action":"list","response_hint":"ok"}`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrIntentModuleUnknown))
}

func TestIntentValidator_RejectsInvalidAction(t *testing.T) {
	sut := services.NewIntentValidator()
	_, err := sut.Validate([]byte(`{"module":"cards","action":"explode","response_hint":"ok"}`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrIntentActionUnknown))
}

func TestIntentValidator_StripsMarkdownFences(t *testing.T) {
	sut := services.NewIntentValidator()
	raw := []byte("```json\n{\"module\":\"cards\",\"action\":\"list\",\"response_hint\":\"ok\"}\n```")

	intent, err := sut.Validate(raw)
	require.NoError(t, err)
	assert.True(t, intent.Module().Equal(valueobjects.IntentModuleCards()))
}

func TestIntentValidator_EmptyOutput(t *testing.T) {
	sut := services.NewIntentValidator()
	_, err := sut.Validate(nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrValidatorEmptyOutput))
}

func TestIntentValidator_InvalidJSON(t *testing.T) {
	sut := services.NewIntentValidator()
	_, err := sut.Validate([]byte("{broken"))
	require.Error(t, err)
}
