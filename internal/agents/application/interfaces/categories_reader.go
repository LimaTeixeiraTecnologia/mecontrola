package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type CategoriesReader interface {
	SearchDictionary(ctx context.Context, term, kind string) (CategorySearchResult, error)
	ResolveForWrite(ctx context.Context, input CategoryWriteRequest) (CategoryWriteDecision, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]Category, error)
}
