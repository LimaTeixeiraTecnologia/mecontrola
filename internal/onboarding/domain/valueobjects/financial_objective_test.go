package valueobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func TestNewFinancialObjective_HappyPath(t *testing.T) {
	t.Parallel()
	got, err := valueobjects.NewFinancialObjective("  fazer   uma viagem  ")
	require.NoError(t, err)
	require.Equal(t, "fazer uma viagem", got.String())
}

func TestNewFinancialObjective_Empty(t *testing.T) {
	t.Parallel()
	_, err := valueobjects.NewFinancialObjective("   ")
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrFinancialObjectiveEmpty))
}

func TestNewFinancialObjective_TooLong(t *testing.T) {
	t.Parallel()
	_, err := valueobjects.NewFinancialObjective(strings.Repeat("a", 281))
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrFinancialObjectiveTooLong))
}
