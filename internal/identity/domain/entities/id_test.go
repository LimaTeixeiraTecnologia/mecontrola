package entities_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

func TestNewID_IsValidUUIDv4(t *testing.T) {
	id := entities.NewID()
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(4), parsed.Version(), "deve ser UUID versão 4")
}

func TestNewID_GeneratesDistinctIDs(t *testing.T) {
	const n = 20
	ids := make(map[string]struct{}, n)
	for range n {
		id := entities.NewID()
		require.NotEmpty(t, id)
		_, duplicate := ids[id]
		assert.False(t, duplicate, "IDs gerados devem ser únicos; duplicata: %s", id)
		ids[id] = struct{}{}
	}
}
