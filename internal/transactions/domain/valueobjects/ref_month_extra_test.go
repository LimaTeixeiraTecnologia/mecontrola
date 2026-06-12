package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestRefMonthEqual(t *testing.T) {
	rm1, _ := valueobjects.NewRefMonth("2026-06")
	rm2, _ := valueobjects.NewRefMonth("2026-06")
	rm3, _ := valueobjects.NewRefMonth("2026-07")

	assert.True(t, rm1.Equal(rm2))
	assert.False(t, rm1.Equal(rm3))
}

func TestRefMonthBefore(t *testing.T) {
	rm1, _ := valueobjects.NewRefMonth("2026-06")
	rm2, _ := valueobjects.NewRefMonth("2026-07")

	assert.True(t, rm1.Before(rm2))
	assert.False(t, rm2.Before(rm1))
}

func TestRefMonthIsZero(t *testing.T) {
	var zero valueobjects.RefMonth
	assert.True(t, zero.IsZero())

	rm, err := valueobjects.NewRefMonth("2026-06")
	require.NoError(t, err)
	assert.False(t, rm.IsZero())
}
