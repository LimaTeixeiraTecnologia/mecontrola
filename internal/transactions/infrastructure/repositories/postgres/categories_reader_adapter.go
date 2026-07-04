package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	catinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/config"
)

type categoriesReaderAdapter struct {
	resolveBySlug       *catusecases.ResolveBySlug
	validateSubcategory *catusecases.ValidateSubcategory
	versionReader       catinterfaces.VersionReader
	o11y                observability.Observability
}

func NewCategoriesReaderAdapter(
	resolveBySlug *catusecases.ResolveBySlug,
	validateSubcategory *catusecases.ValidateSubcategory,
	versionReader catinterfaces.VersionReader,
	o11y observability.Observability,
) config.CategoriesReader {
	return &categoriesReaderAdapter{
		resolveBySlug:       resolveBySlug,
		validateSubcategory: validateSubcategory,
		versionReader:       versionReader,
		o11y:                o11y,
	}
}

func (a *categoriesReaderAdapter) ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "transactions.categories_reader.resolve_roots_by_slug")
	defer span.End()

	rawSlugs := make([]string, 0, len(slugs))
	compoundToRaw := make(map[string]string, len(slugs))
	for _, s := range slugs {
		raw := rawSlug(s)
		rawSlugs = append(rawSlugs, raw)
		compoundToRaw[s] = raw
	}

	resolved, err := a.resolveBySlug.Execute(ctx, rawSlugs)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/categories_reader: resolver raízes: %w", interfaces.ErrCategoryNotFound)
	}

	result := make(map[string]uuid.UUID, len(slugs))
	for compound, raw := range compoundToRaw {
		id, ok := resolved[raw]
		if !ok {
			return nil, fmt.Errorf("transactions/categories_reader: slug %q não encontrado: %w", compound, interfaces.ErrCategoryNotFound)
		}
		result[compound] = id
	}

	return result, nil
}

func (a *categoriesReaderAdapter) ValidateSubcategory(ctx context.Context, id uuid.UUID, expectedParentID uuid.UUID) (interfaces.CategorySnapshot, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "transactions.categories_reader.validate_subcategory")
	defer span.End()

	result, err := a.validateSubcategory.Execute(ctx, id, expectedParentID)
	if err != nil {
		span.RecordError(err)
		return interfaces.CategorySnapshot{}, fmt.Errorf("transactions/categories_reader: validar subcategoria: %w", interfaces.ErrCategoryNotFound)
	}

	parentID := expectedParentID
	return interfaces.CategorySnapshot{
		ID:         id,
		Name:       result.CategoryName,
		Kind:       result.Kind,
		ParentID:   &parentID,
		ParentName: result.ParentName,
	}, nil
}

func (a *categoriesReaderAdapter) EditorialVersion(ctx context.Context) (int64, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "transactions.categories_reader.editorial_version")
	defer span.End()

	v, err := a.versionReader.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("transactions/categories_reader: ler versão editorial: %w", interfaces.ErrCategoryNotFound)
	}

	return v, nil
}

func rawSlug(compound string) string {
	parts := strings.SplitN(compound, ".", 2)
	if len(parts) == 2 {
		return strings.ReplaceAll(parts[1], "_", "-")
	}
	return compound
}
