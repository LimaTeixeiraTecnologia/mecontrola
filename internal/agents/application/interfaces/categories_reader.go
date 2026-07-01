package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type CategoriesReader interface {
	SearchDictionary(ctx context.Context, term, kind string) ([]CategoryCandidate, error)
	ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
}
