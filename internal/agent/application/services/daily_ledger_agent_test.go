package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
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
