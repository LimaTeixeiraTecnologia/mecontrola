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
)

type categoriesReaderAdapter struct {
	searchDictionary        *catusecases.SearchDictionary
	resolveCategoryForWrite *catusecases.ResolveCategoryForWrite
	listCategories          *catusecases.ListCategories
	o11y                    observability.Observability
}

func NewCategoriesReaderAdapter(
	searchDictionary *catusecases.SearchDictionary,
	resolveCategoryForWrite *catusecases.ResolveCategoryForWrite,
	listCategories *catusecases.ListCategories,
	o11y observability.Observability,
) agentsifaces.CategoriesReader {
	return &categoriesReaderAdapter{
		searchDictionary:        searchDictionary,
		resolveCategoryForWrite: resolveCategoryForWrite,
		listCategories:          listCategories,
		o11y:                    o11y,
	}
}

func (a *categoriesReaderAdapter) SearchDictionary(ctx context.Context, term, kind string) (agentsifaces.CategorySearchResult, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.categories_reader.search_dictionary")
	defer span.End()

	k, err := catusecases.ParseKind(kind)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CategorySearchResult{}, fmt.Errorf("agents/binding/categories_reader: kind inválido %q: %w", kind, agentsifaces.ErrCategoriesReaderUnavailable)
	}

	out, err := a.searchDictionary.Execute(ctx, &catinput.SearchDictionaryInput{
		Query: term,
		Kind:  k,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CategorySearchResult{}, fmt.Errorf("agents/binding/categories_reader: buscar dicionário: %w", err)
	}

	candidates := make([]agentsifaces.CategoryCandidate, 0, len(out.Candidates))
	for _, c := range out.Candidates {
		candidates = append(candidates, agentsifaces.CategoryCandidate{
			CategoryID:     c.CategoryID,
			RootCategoryID: c.RootCategoryID,
			Path:           c.Path,
			MatchedTerm:    c.MatchedTerm,
			SignalType:     c.SignalType,
			Confidence:     c.Confidence,
			MatchQuality:   c.MatchQuality,
			Score:          c.Score,
			IsAmbiguous:    c.IsAmbiguous,
			MatchReason:    c.MatchReason,
		})
	}
	outcome, err := agentsifaces.ParseClassifyOutcome(out.Outcome.String())
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CategorySearchResult{}, fmt.Errorf("agents/binding/categories_reader: outcome inválido %q: %w", out.Outcome.String(), err)
	}
	return agentsifaces.CategorySearchResult{
		Outcome:    outcome,
		Version:    out.Version,
		HasMore:    out.HasMore,
		Candidates: candidates,
	}, nil
}

func (a *categoriesReaderAdapter) ResolveForWrite(ctx context.Context, input agentsifaces.CategoryWriteRequest) (agentsifaces.CategoryWriteDecision, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.categories_reader.resolve_for_write")
	defer span.End()

	kind, err := catusecases.ParseKind(input.Kind.String())
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CategoryWriteDecision{}, fmt.Errorf("agents/binding/categories_reader: kind inválido %q: %w", input.Kind.String(), agentsifaces.ErrCategoriesReaderUnavailable)
	}

	out, err := a.resolveCategoryForWrite.Execute(ctx, &catinput.ResolveCategoryForWriteInput{
		RootCategoryID:  input.RootCategoryID,
		SubcategoryID:   input.SubcategoryID,
		Kind:            kind,
		ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CategoryWriteDecision{}, fmt.Errorf("agents/binding/categories_reader: resolver para escrita: %w", err)
	}

	decisionKind, err := agentsifaces.ParseCategoryKind(out.Kind)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CategoryWriteDecision{}, fmt.Errorf("agents/binding/categories_reader: kind de decisão inválido %q: %w", out.Kind, err)
	}
	return agentsifaces.CategoryWriteDecision{
		RootCategoryID:   out.RootCategoryID,
		SubcategoryID:    out.SubcategoryID,
		Kind:             decisionKind,
		Path:             out.Path,
		RootSlug:         out.RootSlug,
		SubcategorySlug:  out.SubcategorySlug,
		EditorialVersion: out.EditorialVersion,
		Deprecated:       out.Deprecated,
	}, nil
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

func (a *categoriesReaderAdapter) CatalogVersion(ctx context.Context) (int64, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.categories_reader.catalog_version")
	defer span.End()

	out, err := a.listCategories.Execute(ctx, &catinput.ListCategoriesInput{})
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("agents/binding/categories_reader: versão do catálogo: %w", err)
	}
	return out.Version, nil
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
