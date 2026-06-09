package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
)

type ListCategories struct {
	repo    interfaces.CategoryRepository
	version interfaces.VersionReader
	o11y    observability.Observability
}

func NewListCategories(repo interfaces.CategoryRepository, version interfaces.VersionReader, o11y observability.Observability) *ListCategories {
	return &ListCategories{repo: repo, version: version, o11y: o11y}
}

func (uc *ListCategories) Execute(ctx context.Context, in *input.ListCategoriesInput) (*output.ListCategoriesOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.list")
	defer span.End()

	version, err := uc.version.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("ler versao: %w", err)
	}

	query := interfaces.CategoryQuery{
		IncludeDeprecated: in.IncludeDeprecated,
	}
	if in.Kind != nil {
		query.Kind = *in.Kind
	}
	if in.ParentID != nil {
		query.ParentID = in.ParentID
	}

	categories, err := uc.repo.List(ctx, query)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("listar categorias: %w", err)
	}

	if in.ParentID != nil {
		return uc.buildFlatList(categories, version), nil
	}

	return uc.buildTree(categories, version), nil
}

func (uc *ListCategories) buildFlatList(categories []entities.Category, version int64) *output.ListCategoriesOutput {
	result := make([]output.CategoryTreeOutput, 0, len(categories))
	for _, c := range categories {
		result = append(result, output.CategoryTreeOutput{
			ID:             c.ID,
			Slug:           c.Slug,
			Name:           c.Name,
			Kind:           c.Kind.String(),
			ParentID:       c.ParentID,
			AllocationType: c.AllocationType.String(),
			DeprecatedAt:   c.GetDeprecatedAt(),
			Version:        version,
		})
	}
	return &output.ListCategoriesOutput{Categories: result, Version: version}
}

func (uc *ListCategories) buildTree(categories []entities.Category, version int64) *output.ListCategoriesOutput {
	roots := make([]entities.Category, 0)
	children := make(map[string][]entities.Category)

	for _, c := range categories {
		if c.IsRoot() {
			roots = append(roots, c)
		} else if c.ParentID != nil {
			parentID := c.ParentID.String()
			children[parentID] = append(children[parentID], c)
		}
	}

	result := make([]output.CategoryTreeOutput, 0, len(roots))
	for _, root := range roots {
		subs := children[root.ID.String()]
		subOutputs := make([]output.CategoryOutput, 0, len(subs))
		for _, s := range subs {
			subOutputs = append(subOutputs, output.CategoryOutput{
				ID:             s.ID,
				Slug:           s.Slug,
				Name:           s.Name,
				Kind:           s.Kind.String(),
				ParentID:       s.ParentID,
				AllocationType: s.AllocationType.String(),
				DeprecatedAt:   s.GetDeprecatedAt(),
				Version:        version,
			})
		}

		result = append(result, output.CategoryTreeOutput{
			ID:             root.ID,
			Slug:           root.Slug,
			Name:           root.Name,
			Kind:           root.Kind.String(),
			AllocationType: root.AllocationType.String(),
			DeprecatedAt:   root.GetDeprecatedAt(),
			Subcategories:  subOutputs,
			Version:        version,
		})
	}

	return &output.ListCategoriesOutput{Categories: result, Version: version}
}
