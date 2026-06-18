package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

func TestNewKiwifySubscriptionID(t *testing.T) {
	t.Run("deve aceitar valor valido removendo espacos externos", func(t *testing.T) {
		id, err := valueobjects.NewKiwifySubscriptionID("  kiwify-sub-001  ")

		require.NoError(t, err)
		require.Equal(t, "kiwify-sub-001", id.String())
	})

	t.Run("deve rejeitar valor vazio", func(t *testing.T) {
		_, err := valueobjects.NewKiwifySubscriptionID("   ")

		require.ErrorIs(t, err, valueobjects.ErrKiwifySubscriptionIDEmpty)
	})
}
