package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestMoneyEqual(t *testing.T) {
	m1, err := valueobjects.NewMoney(100)
	require.NoError(t, err)
	m2, err := valueobjects.NewMoney(100)
	require.NoError(t, err)
	m3, err := valueobjects.NewMoney(200)
	require.NoError(t, err)

	assert.True(t, m1.Equal(m2))
	assert.False(t, m1.Equal(m3))
}
