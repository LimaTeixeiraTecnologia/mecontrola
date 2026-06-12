package usecases

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
)

var ErrCategoryNotFound = errors.New("category not found")

type GetCategory struct {
	repo     interfaces.CategoryRepository
	version  interfaces.VersionReader
	collator *services.PTBRCollator
	o11y     observability.Observability
}

func NewGetCategory(repo interfaces.CategoryRepository, version interfaces.VersionReader, collator *services.PTBRCollator, o11y observability.Observability) *GetCategory {
	return &GetCategory{repo: repo, version: version, collator: collator, o11y: o11y}
}

func (uc *GetCategory) Execute(ctx context.Context, in *input.GetCategoryInput) (*output.CategoryDetailOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.get")
	defer span.End()

	version, err := uc.version.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("ler versao: %w", err)
	}

	category, err := uc.repo.GetByID(ctx, in.ID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("buscar categoria: %w", err)
	}

	if !category.IsActive() && !in.IncludeDeprecated {
		return nil, ErrCategoryNotFound
	}

	result := &output.CategoryDetailOutput{
		ID:             category.ID,
		Slug:           category.Slug,
		Name:           category.Name,
		Kind:           category.Kind.String(),
		ParentID:       category.ParentID,
		AllocationType: category.AllocationType.String(),
		DeprecatedAt:   category.GetDeprecatedAt(),
		Version:        version,
	}

	if category.IsRoot() {
		subs, err := uc.repo.List(ctx, interfaces.CategoryQuery{
			Kind:     category.Kind,
			ParentID: &category.ID,
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("listar subcategorias: %w", err)
		}
		result.Subcategories = uc.buildSubcategoryOutputs(subs, version)
		result.Path = category.Name
	} else {
		path, err := uc.buildPath(ctx, category)
		if err != nil {
			return nil, err
		}
		result.Path = path
	}

	return result, nil
}

func (uc *GetCategory) buildPath(ctx context.Context, category entities.Category) (string, error) {
	if category.ParentID == nil {
		return category.Name, nil
	}

	parent, err := uc.repo.GetByID(ctx, *category.ParentID)
	if err != nil {
		_, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.get.buildPath")
		defer span.End()
		span.RecordError(err)
		return "", fmt.Errorf("buscar categoria pai: %w", err)
	}

	if parent.Kind != category.Kind {
		return "", fmt.Errorf("categoria %s aponta para pai de kind diferente", category.ID)
	}

	return parent.Name + " > " + category.Name, nil
}

func (uc *GetCategory) buildSubcategoryOutputs(categories []entities.Category, version int64) []output.CategoryOutput {
	sort.Slice(categories, func(i, j int) bool {
		return uc.collator.Less(categories[i].Name, categories[j].Name)
	})

	result := make([]output.CategoryOutput, 0, len(categories))
	for _, c := range categories {
		result = append(result, output.CategoryOutput{
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
	return result
}
