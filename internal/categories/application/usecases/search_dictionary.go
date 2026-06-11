package usecases

import (
	"context"
	"fmt"
	"strings"

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

	if err := in.Validate(); err != nil {
		return nil, err
	}

	version, err := uc.version.Current(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("ler versao: %w", err)
	}

	entries, err := uc.repo.Search(ctx, interfaces.DictionarySearchQuery{
		Kind:  in.Kind,
		Term:  strings.TrimSpace(in.Query),
		Limit: searchCandidateLimit,
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
	categories := make(map[uuid.UUID]entities.Category)
	for _, e := range entries {
		if _, exists := categories[e.CategoryID]; !exists {
			cat, err := uc.categoryRepo.GetByID(ctx, e.CategoryID)
			if err != nil {
				continue
			}
			categories[e.CategoryID] = cat
			if cat.ParentID != nil {
				parent, err := uc.categoryRepo.GetByID(ctx, *cat.ParentID)
				if err == nil {
					categories[parent.ID] = parent
				}
			}
		}
	}
	return categories, nil
}
