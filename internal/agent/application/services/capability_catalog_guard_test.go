package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
)

func TestBuildCatalogCoversAllRoutableKinds(t *testing.T) {
	t.Parallel()

	catalog, err := capability.BuildCatalog()
	require.NoError(t, err)

	for _, kind := range routableKinds() {
		_, ok := catalog.Lookup(kind)
		require.True(t, ok, "catalogo sem spec para kind roteavel: %s", kind.String())
	}
}

func TestBuildCatalogMatchesRegistryWorkflowOwners(t *testing.T) {
	t.Parallel()

	catalog, err := capability.BuildCatalog()
	require.NoError(t, err)

	registry, err := (&DailyLedgerAgent{}).buildRegistry()
	require.NoError(t, err)

	for _, kind := range routableKinds() {
		spec, ok := catalog.Lookup(kind)
		require.True(t, ok, "catalogo sem spec para kind roteavel: %s", kind.String())

		owner, ok := registry.Resolve(kind)
		require.True(t, ok, "registry sem owner para kind roteavel: %s", kind.String())
		require.Equal(t, owner.ID(), spec.WorkflowID, "workflow divergente para kind %s", kind.String())
	}
}
