package usecases

import (
	"context"
	"fmt"
	"maps"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

const searchCandidateLimit = 100

type SearchDictionary struct {
	repo         interfaces.DictionaryRepository
	categoryRepo interfaces.CategoryRepository
	version      interfaces.VersionReader
	resolver     *services.CandidateResolver
	o11y         observability.Observability
}

func NewSearchDictionary(repo interfaces.DictionaryRepository, categoryRepo interfaces.CategoryRepository, version interfaces.VersionReader, resolver *services.CandidateResolver, o11y observability.Observability) *SearchDictionary {
	return &SearchDictionary{repo: repo, categoryRepo: categoryRepo, version: version, resolver: resolver, o11y: o11y}
}

func (uc *SearchDictionary) Execute(ctx context.Context, in *input.SearchDictionaryInput) (*output.DictionarySearchOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "categories.usecase.search")
	defer span.End()

	if !in.Kind.IsValid() {
		return nil, valueobjects.ErrInvalidKind
	}
	query, err := in.SearchQuery()
	if err != nil {
		return nil, err
	}

	version, err := uc.version.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("ler versao: %w", err)
	}

	entries, err := uc.repo.Search(ctx, interfaces.DictionarySearchQuery{
		Kind:              in.Kind,
		Term:              query.Trimmed(),
		Limit:             searchCandidateLimit,
		IncludeDeprecated: false,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("buscar dicionario: %w", err)
	}

	if len(entries) == 0 {
		return &output.DictionarySearchOutput{
			Result:  "no_match",
			Outcome: valueobjects.SearchOutcomeNoMatch,
			Version: version,
		}, nil
	}

	categories, err := uc.buildCategoryMap(ctx, entries)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("buscar categorias: %w", err)
	}
	candidates, hasMore := uc.resolver.Resolve(entries, categories)

	outcome := valueobjects.ClassifyOutcome(len(candidates))
	if outcome == valueobjects.SearchOutcomeNoMatch {
		return &output.DictionarySearchOutput{
			Result:  "no_match",
			Outcome: outcome,
			Version: version,
		}, nil
	}

	candidateOutputs := make([]output.CandidateOutput, 0, len(candidates))
	for _, c := range candidates {
		candidateOutputs = append(candidateOutputs, output.NewCandidateOutputFromService(c))
	}

	return &output.DictionarySearchOutput{
		Result:        "candidates",
		Candidates:    candidateOutputs,
		HasMore:       hasMore,
		SignalTypeTop: candidates[0].SignalType.String(),
		Outcome:       outcome,
		Version:       version,
	}, nil
}

func (uc *SearchDictionary) buildCategoryMap(ctx context.Context, entries []entities.DictionaryEntry) (map[uuid.UUID]entities.Category, error) {
	categoryIDs := distinctCategoryIDs(entries)
	categories, err := uc.fetchByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}

	parentIDs := missingParentIDs(categories)
	if len(parentIDs) == 0 {
		return categories, nil
	}

	parents, err := uc.fetchByIDs(ctx, parentIDs)
	if err != nil {
		return nil, err
	}
	maps.Copy(categories, parents)
	return categories, nil
}

func (uc *SearchDictionary) fetchByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]entities.Category, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]entities.Category{}, nil
	}
	rows, err := uc.categoryRepo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]entities.Category, len(rows))
	for _, c := range rows {
		out[c.ID] = c
	}
	return out, nil
}

func distinctCategoryIDs(entries []entities.DictionaryEntry) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(entries))
	ids := make([]uuid.UUID, 0, len(entries))
	for _, e := range entries {
		if _, ok := seen[e.CategoryID]; ok {
			continue
		}
		seen[e.CategoryID] = struct{}{}
		ids = append(ids, e.CategoryID)
	}
	return ids
}

func missingParentIDs(categories map[uuid.UUID]entities.Category) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(categories))
	ids := make([]uuid.UUID, 0, len(categories))
	for _, c := range categories {
		if c.ParentID == nil {
			continue
		}
		pid := *c.ParentID
		if _, ok := categories[pid]; ok {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		ids = append(ids, pid)
	}
	return ids
}
