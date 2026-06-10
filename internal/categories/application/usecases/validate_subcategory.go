package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
)

var ErrSubcategoryNotRoot = errors.New("categories: subcategoria é uma raiz, não uma subcategoria")

type ValidateSubcategoryResult struct {
	ParentSlug string
	Deprecated bool
}

type ValidateSubcategory struct {
	repo interfaces.CategoryRepository
	o11y observability.Observability
}

func NewValidateSubcategory(repo interfaces.CategoryRepository, o11y observability.Observability) *ValidateSubcategory {
	return &ValidateSubcategory{repo: repo, o11y: o11y}
}

func (uc *ValidateSubcategory) Execute(ctx context.Context, id uuid.UUID) (ValidateSubcategoryResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.validate_subcategory")
	defer span.End()

	category, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, interfaces.ErrNotFound) {
			return ValidateSubcategoryResult{}, ErrCategoryNotFound
		}
		return ValidateSubcategoryResult{}, fmt.Errorf("categories/validate_subcategory: buscar categoria: %w", err)
	}

	if category.IsRoot() {
		return ValidateSubcategoryResult{}, ErrSubcategoryNotRoot
	}

	parent, err := uc.repo.GetByID(ctx, *category.ParentID)
	if err != nil {
		span.RecordError(err)
		return ValidateSubcategoryResult{}, fmt.Errorf("categories/validate_subcategory: buscar categoria pai: %w", err)
	}

	return ValidateSubcategoryResult{
		ParentSlug: uc.buildRootSlug(parent),
		Deprecated: !category.IsActive(),
	}, nil
}

func (uc *ValidateSubcategory) buildRootSlug(parent entities.Category) string {
	return parent.Kind.String() + "." + uc.normalizeSlug(parent.Slug)
}

func (uc *ValidateSubcategory) normalizeSlug(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' {
			result = append(result, '_')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
