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

const (
	searchCandidateLimit = 100
	fuzzyMinSimilarity   = 0.4
)

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

	if err := in.Validate(); err != nil {
		return nil, err
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

	scored, err := uc.resolveEntries(ctx, in.Kind, query)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	if len(scored) == 0 {
		return &output.DictionarySearchOutput{
			Result:  "no_match",
			Outcome: valueobjects.SearchOutcomeNoMatch,
			Version: version,
		}, nil
	}

	entries := make([]entities.DictionaryEntry, 0, len(scored))
	for _, se := range scored {
		entries = append(entries, se.Entry)
	}

	categories, err := uc.buildCategoryMap(ctx, entries)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("buscar categorias: %w", err)
	}
	candidates, hasMore := uc.resolver.ResolveScored(scored, categories)
	if len(candidates) == 0 {
		return &output.DictionarySearchOutput{
			Result:  "no_match",
			Outcome: valueobjects.SearchOutcomeNoMatch,
			Version: version,
		}, nil
	}

	scores := make([]valueobjects.MatchScore, 0, len(candidates))
	for _, c := range candidates {
		scores = append(scores, c.Score)
	}
	outcome := valueobjects.ClassifyByScore(scores)

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

func (uc *SearchDictionary) resolveEntries(ctx context.Context, kind valueobjects.Kind, query valueobjects.SearchQuery) ([]services.ScoredEntry, error) {
	exact, err := uc.repo.Search(ctx, interfaces.DictionarySearchQuery{
		Kind:  kind,
		Term:  query.Trimmed(),
		Limit: searchCandidateLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("buscar dicionario: %w", err)
	}
	if len(exact) > 0 {
		return scoredEntries(exact, valueobjects.MatchQualityExact), nil
	}

	tokens := query.Tokens()
	if len(tokens) == 0 {
		return nil, nil
	}

	byToken, err := uc.repo.SearchTokens(ctx, interfaces.DictionaryTokenSearchQuery{
		Kind:   kind,
		Tokens: tokens,
		Limit:  searchCandidateLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("buscar dicionario por token: %w", err)
	}
	if len(byToken) > 0 {
		return scoredEntries(byToken, valueobjects.MatchQualityToken), nil
	}

	byFuzzy, err := uc.repo.SearchFuzzy(ctx, interfaces.DictionaryFuzzySearchQuery{
		Kind:          kind,
		Tokens:        tokens,
		MinSimilarity: fuzzyMinSimilarity,
		Limit:         searchCandidateLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("buscar dicionario fuzzy: %w", err)
	}
	if len(byFuzzy) > 0 {
		return scoredEntries(byFuzzy, valueobjects.MatchQualityFuzzy), nil
	}

	return nil, nil
}

func scoredEntries(entries []entities.DictionaryEntry, quality valueobjects.MatchQuality) []services.ScoredEntry {
	scored := make([]services.ScoredEntry, 0, len(entries))
	for _, e := range entries {
		scored = append(scored, services.ScoredEntry{Entry: e, Quality: quality})
	}
	return scored
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
