package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestIntentToOperationKindRequiresConfirmationInCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := capability.BuildCatalog()
	require.NoError(t, err)

	for kind := range intentToOperationKind {
		spec, ok := catalog.Lookup(kind)
		require.True(t, ok, "catalogo sem spec para kind destrutivo: %s", kind.String())
		require.True(t, spec.RequiresConfirmation, "kind destrutivo sem requires_confirmation: %s", kind.String())
	}
}

func TestIsDestructiveKindFailsSafeWhenCatalogNil(t *testing.T) {
	t.Parallel()

	agent := &DailyLedgerAgent{catalog: nil}
	for kind := range intentToOperationKind {
		require.True(t, agent.isDestructiveKind(kind), "kind destrutivo deve exigir confirmacao quando catalog e nil: %s", kind.String())
	}
	require.True(t, agent.isDestructiveKind(intent.KindUnknown), "kind desconhecido deve ser tratado como destrutivo quando catalog e nil")
}

func TestIsDestructiveKindReturnsTrueForUnknownKind(t *testing.T) {
	t.Parallel()

	catalog, err := capability.BuildCatalog()
	require.NoError(t, err)

	agent := &DailyLedgerAgent{catalog: catalog}
	require.True(t, agent.isDestructiveKind(intent.Kind(9999)), "kind nao catalogado deve ser tratado como destrutivo")
}

func TestIsDestructiveKindFailsSafeWhenAgentNil(t *testing.T) {
	t.Parallel()

	var agent *DailyLedgerAgent
	for kind := range intentToOperationKind {
		require.True(t, agent.isDestructiveKind(kind), "kind destrutivo deve exigir confirmacao quando agent e nil: %s", kind.String())
	}
	require.True(t, agent.isDestructiveKind(intent.KindUnknown), "kind desconhecido deve ser tratado como destrutivo quando agent e nil")
}
