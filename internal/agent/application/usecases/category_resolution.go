package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

func resolveCategoryCandidate(
	ctx context.Context,
	resolver CategoryResolver,
	resolveBad observability.Counter,
	hint string,
	kind categoriesvo.Kind,
) (categoriesoutput.CandidateOutput, string, error) {
	result, err := resolver.Execute(ctx, &categoriesinput.SearchDictionaryInput{Query: hint, Kind: kind})
	if err != nil {
		resolveBad.Add(ctx, 1, observability.String("reason", "resolver_failed"))
		return categoriesoutput.CandidateOutput{}, "", fmt.Errorf("agent: resolver categoria: %w", err)
	}
	if result == nil || len(result.Candidates) == 0 {
		resolveBad.Add(ctx, 1, observability.String("reason", "no_match"))
		return categoriesoutput.CandidateOutput{}, "", ErrLogTransactionCategoryNotFound
	}
	top := result.Candidates[0]
	if top.IsAmbiguous && len(result.Candidates) > 1 {
		resolveBad.Add(ctx, 1, observability.String("reason", "ambiguous"))
		return categoriesoutput.CandidateOutput{}, "", newCategoryAmbiguousError(hint, result.Candidates)
	}
	return top, top.Path, nil
}

func candidateSubcategoryUUID(candidate categoriesoutput.CandidateOutput) *uuid.UUID {
	if candidate.CategoryID == uuid.Nil || candidate.CategoryID == candidate.RootCategoryID {
		return nil
	}
	v := candidate.CategoryID
	return &v
}

func resolveHint(categoryHint, merchant string) string {
	hint := strings.TrimSpace(categoryHint)
	if hint == "" {
		hint = strings.TrimSpace(merchant)
	}
	return hint
}
