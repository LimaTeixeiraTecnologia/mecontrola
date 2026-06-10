package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrCategoriesReaderUnavailable = errors.New("budgets: categories reader indisponível")

type CategoriesReader interface {
	ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
	ValidateExpenseSubcategory(ctx context.Context, id uuid.UUID) (rootSlug string, deprecated bool, err error)
	EditorialVersion(ctx context.Context) (int64, error)
}
