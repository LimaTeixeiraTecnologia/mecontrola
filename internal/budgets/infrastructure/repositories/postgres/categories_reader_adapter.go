package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	catinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
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
) budgetsinterfaces.CategoriesReader {
	return &categoriesReaderAdapter{
		resolveBySlug:       resolveBySlug,
		validateSubcategory: validateSubcategory,
		versionReader:       versionReader,
		o11y:                o11y,
	}
}

func (a *categoriesReaderAdapter) ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "budgets.categories_reader.resolve_roots_by_slug")
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
		return nil, fmt.Errorf("budgets/categories_reader: resolver raízes: %w", budgetsinterfaces.ErrCategoriesReaderUnavailable)
	}

	result := make(map[string]uuid.UUID, len(slugs))
	for compound, raw := range compoundToRaw {
		id, ok := resolved[raw]
		if !ok {
			return nil, fmt.Errorf("budgets/categories_reader: slug %q não encontrado: %w", compound, budgetsinterfaces.ErrCategoriesReaderUnavailable)
		}
		result[compound] = id
	}

	return result, nil
}

func (a *categoriesReaderAdapter) ValidateExpenseSubcategory(ctx context.Context, id uuid.UUID) (string, bool, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "budgets.categories_reader.validate_expense_subcategory")
	defer span.End()

	result, err := a.validateSubcategory.Execute(ctx, id, uuid.Nil)
	if err != nil {
		span.RecordError(err)
		return "", false, fmt.Errorf("budgets/categories_reader: validar subcategoria: %w", budgetsinterfaces.ErrCategoriesReaderUnavailable)
	}

	return result.ParentSlug, result.Deprecated, nil
}

func (a *categoriesReaderAdapter) EditorialVersion(ctx context.Context) (int64, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "budgets.categories_reader.editorial_version")
	defer span.End()

	v, err := a.versionReader.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("budgets/categories_reader: ler versão editorial: %w", budgetsinterfaces.ErrCategoriesReaderUnavailable)
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
