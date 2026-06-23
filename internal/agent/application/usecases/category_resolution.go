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

func newMatchScoreHistogram(o11y observability.Observability) observability.Histogram {
	return o11y.Metrics().HistogramWithBuckets(
		"agent_category_match_score",
		"Distribuição do score de match de categoria por outcome",
		"1",
		[]float64{0.3, 0.4, 0.55, 0.65, 0.7, 0.79, 0.8, 0.9, 1},
	)
}

func resolveCategoryCandidate(
	ctx context.Context,
	resolver CategoryResolver,
	resolveBad observability.Counter,
	scoreHistogram observability.Histogram,
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
		recordMatchScore(ctx, scoreHistogram, top.Score, "ambiguous")
		resolveBad.Add(ctx, 1, observability.String("reason", "ambiguous"))
		return categoriesoutput.CandidateOutput{}, "", newCategoryAmbiguousError(hint, result.Candidates)
	}
	switch {
	case top.Score >= categoriesvo.ScoreAutoThreshold || isUnequivocalExactMatch(top):
		recordMatchScore(ctx, scoreHistogram, top.Score, "auto_logged")
		return top, top.Path, nil
	case top.Score >= categoriesvo.ScoreConfirmThreshold:
		recordMatchScore(ctx, scoreHistogram, top.Score, "needs_confirmation")
		resolveBad.Add(ctx, 1, observability.String("reason", "needs_confirmation"))
		return categoriesoutput.CandidateOutput{}, "", newCategoryNeedsConfirmationError(hint, result.Candidates)
	default:
		recordMatchScore(ctx, scoreHistogram, top.Score, "low_score")
		resolveBad.Add(ctx, 1, observability.String("reason", "low_score"))
		return categoriesoutput.CandidateOutput{}, "", ErrLogTransactionCategoryNotFound
	}
}

func isUnequivocalExactMatch(candidate categoriesoutput.CandidateOutput) bool {
	return candidate.MatchQuality == categoriesvo.MatchQualityExact.String() &&
		candidate.Confidence == categoriesvo.ConfidenceHigh.String()
}

func recordMatchScore(ctx context.Context, h observability.Histogram, score float64, outcome string) {
	if h == nil {
		return
	}
	h.Record(ctx, score, observability.String("outcome", outcome))
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
