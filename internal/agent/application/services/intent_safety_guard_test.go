package services_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func mustAction(t *testing.T, raw string) valueobjects.IntentAction {
	t.Helper()
	action, err := valueobjects.NewIntentAction(raw)
	require.NoError(t, err)
	return action
}

func mustSafeIntent(
	t *testing.T,
	module valueobjects.IntentModule,
	action valueobjects.IntentAction,
	filters string,
	payload string,
) entities.IntentResult {
	t.Helper()
	intent, err := entities.NewIntentResult(
		module,
		action,
		json.RawMessage(filters),
		json.RawMessage(payload),
		"ok",
	)
	require.NoError(t, err)
	return intent
}

func TestIntentSafetyGuard_Validate(t *testing.T) {
	sut := services.NewIntentSafetyGuard()

	t.Run("allows explicit card update", func(t *testing.T) {
		intent := mustSafeIntent(
			t,
			valueobjects.IntentModuleCards(),
			mustAction(t, "update"),
			`{}`,
			`{"id":"3b3b3b3b-0000-0000-0000-000000000001","limit_cents":500000}`,
		)
		err := sut.Validate("atualizar o limite do cartao agora", intent)
		require.NoError(t, err)
	})

	t.Run("blocks card delete without confirmation", func(t *testing.T) {
		intent := mustSafeIntent(
			t,
			valueobjects.IntentModuleCards(),
			mustAction(t, "delete"),
			`{}`,
			`{"id":"3b3b3b3b-0000-0000-0000-000000000001"}`,
		)
		err := sut.Validate("remover meu cartao nubank", intent)
		require.Error(t, err)
		assert.True(t, errors.Is(err, services.ErrIntentSafetyMissingConfirmation))
	})

	t.Run("blocks transaction create without explicit mutation verb", func(t *testing.T) {
		intent := mustSafeIntent(
			t,
			valueobjects.IntentModuleTransactions(),
			valueobjects.IntentActionCreate(),
			`{}`,
			`{"amount":58,"type":"expense","category_id":"3b3b3b3b-0000-0000-0000-000000000001"}`,
		)
		err := sut.Validate("gastei 58 no ifood", intent)
		require.Error(t, err)
		assert.True(t, errors.Is(err, services.ErrIntentSafetyMissingMutationVerb))
	})

	t.Run("blocks get without lookup id", func(t *testing.T) {
		intent := mustSafeIntent(
			t,
			valueobjects.IntentModuleCards(),
			valueobjects.IntentActionGet(),
			`{"nickname":"nubank"}`,
			`{}`,
		)
		err := sut.Validate("me mostra esse cartao", intent)
		require.Error(t, err)
		assert.True(t, errors.Is(err, services.ErrIntentSafetyMissingLookupField))
	})

	t.Run("blocks unsupported budget operation", func(t *testing.T) {
		intent := mustSafeIntent(
			t,
			valueobjects.IntentModuleBudgets(),
			valueobjects.IntentActionCreate(),
			`{}`,
			`{"operation":"transfer","competence":"2026-06"}`,
		)
		err := sut.Validate("criar transferencia no orcamento", intent)
		require.Error(t, err)
		assert.True(t, errors.Is(err, services.ErrIntentSafetyInvalidOperation))
	})
}
