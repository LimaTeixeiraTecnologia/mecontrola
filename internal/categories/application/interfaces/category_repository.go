package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategoryQuery struct {
	Kind              valueobjects.Kind
	ParentID          *uuid.UUID
	IncludeDeprecated bool
}

type CategoryRepository interface {
	List(ctx context.Context, q CategoryQuery) ([]entities.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (entities.Category, error)
}
