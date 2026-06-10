package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ResolveBySlug struct {
	repo interfaces.CategoryRepository
	o11y observability.Observability
}

func NewResolveBySlug(repo interfaces.CategoryRepository, o11y observability.Observability) *ResolveBySlug {
	return &ResolveBySlug{repo: repo, o11y: o11y}
}

func (uc *ResolveBySlug) Execute(ctx context.Context, slugs []string) (map[string]uuid.UUID, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.resolve_by_slug")
	defer span.End()

	categories, err := uc.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		IncludeDeprecated: false,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("categories/resolve_by_slug: listar categorias: %w", err)
	}

	index := make(map[string]uuid.UUID, len(categories))
	for _, c := range categories {
		if c.IsRoot() {
			index[c.Slug] = c.ID
		}
	}

	result := make(map[string]uuid.UUID, len(slugs))
	for _, slug := range slugs {
		id, ok := index[slug]
		if !ok {
			return nil, fmt.Errorf("categories/resolve_by_slug: slug %q não encontrado: %w", slug, ErrCategoryNotFound)
		}
		result[slug] = id
	}

	return result, nil
}
