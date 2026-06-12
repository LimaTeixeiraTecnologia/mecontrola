package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrCategoryNotFound = errors.New("transactions: categoria não encontrada")

type CategoryValidator interface {
	Validate(ctx context.Context, categoryID uuid.UUID, subcategoryID *uuid.UUID) (CategorySnapshot, error)
}
