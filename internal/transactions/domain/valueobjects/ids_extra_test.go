package valueobjects_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestUserIDFromUUID(t *testing.T) {
	id := uuid.New()
	u := valueobjects.UserIDFromUUID(id)
	assert.Equal(t, id, u.UUID())
}

func TestCardIDFromUUID(t *testing.T) {
	id := uuid.New()
	c := valueobjects.CardIDFromUUID(id)
	assert.Equal(t, id, c.UUID())
}

func TestCategoryIDFromUUID(t *testing.T) {
	id := uuid.New()
	c := valueobjects.CategoryIDFromUUID(id)
	assert.Equal(t, id, c.UUID())
}

func TestSubcategoryIDFromUUID(t *testing.T) {
	id := uuid.New()
	s := valueobjects.SubcategoryIDFromUUID(id)
	assert.Equal(t, id, s.UUID())
}
