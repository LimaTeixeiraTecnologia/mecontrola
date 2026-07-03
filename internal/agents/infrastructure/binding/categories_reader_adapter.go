package binding

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	catinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	catoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	catvos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type categoriesReaderAdapter struct {
	searchDictionary *catusecases.SearchDictionary
	resolveBySlug    *catusecases.ResolveBySlug
	listCategories   *catusecases.ListCategories
	o11y             observability.Observability
}

func NewCategoriesReaderAdapter(
	searchDictionary *catusecases.SearchDictionary,
	resolveBySlug *catusecases.ResolveBySlug,
	listCategories *catusecases.ListCategories,
	o11y observability.Observability,
) agentsifaces.CategoriesReader {
	return &categoriesReaderAdapter{
		searchDictionary: searchDictionary,
		resolveBySlug:    resolveBySlug,
		listCategories:   listCategories,
		o11y:             o11y,
	}
}

func (a *categoriesReaderAdapter) SearchDictionary(ctx context.Context, term, kind string) ([]agentsifaces.CategoryCandidate, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.categories_reader.search_dictionary")
	defer span.End()

	k, err := catvos.ParseKind(kind)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/categories_reader: kind inválido %q: %w", kind, agentsifaces.ErrCategoriesReaderUnavailable)
	}

	out, err := a.searchDictionary.Execute(ctx, &catinput.SearchDictionaryInput{
		Query: term,
		Kind:  k,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/categories_reader: buscar dicionário: %w", err)
	}

	candidates := make([]agentsifaces.CategoryCandidate, 0, len(out.Candidates))
	for _, c := range out.Candidates {
		candidates = append(candidates, agentsifaces.CategoryCandidate{
			CategoryID:     c.CategoryID,
			RootCategoryID: c.RootCategoryID,
			Path:           c.Path,
			MatchedTerm:    c.MatchedTerm,
			Score:          c.Score,
			IsAmbiguous:    c.IsAmbiguous,
		})
	}
	return candidates, nil
}

func (a *categoriesReaderAdapter) ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.categories_reader.resolve_roots_by_slug")
	defer span.End()

	result, err := a.resolveBySlug.Execute(ctx, slugs)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/categories_reader: resolver slugs: %w", err)
	}
	return result, nil
}

func (a *categoriesReaderAdapter) ListCategories(ctx context.Context, _ uuid.UUID) ([]agentsifaces.Category, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.categories_reader.list_categories")
	defer span.End()

	out, err := a.listCategories.Execute(ctx, &catinput.ListCategoriesInput{})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/categories_reader: listar categorias: %w", err)
	}

	result := make([]agentsifaces.Category, 0, len(out.Categories))
	for _, c := range out.Categories {
		result = append(result, mapCategoryTree(c))
	}
	return result, nil
}

func mapCategoryTree(c catoutput.CategoryTreeOutput) agentsifaces.Category {
	subs := make([]agentsifaces.Category, 0, len(c.Subcategories))
	for _, s := range c.Subcategories {
		subs = append(subs, agentsifaces.Category{
			ID:       s.ID,
			Slug:     s.Slug,
			Name:     s.Name,
			Kind:     s.Kind,
			ParentID: s.ParentID,
		})
	}
	return agentsifaces.Category{
		ID:             c.ID,
		Slug:           c.Slug,
		Name:           c.Name,
		Kind:           c.Kind,
		ParentID:       c.ParentID,
		AllocationType: c.AllocationType,
		Subcategories:  subs,
	}
}
