package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var ErrNotFound = errors.New("categories: not found")

type CategoryQuery struct {
	Kind              valueobjects.Kind
	ParentID          *uuid.UUID
	IncludeDeprecated bool
}

type CategoryRepository interface {
	List(ctx context.Context, q CategoryQuery) ([]entities.Category, error)
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]entities.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (entities.Category, error)
}
