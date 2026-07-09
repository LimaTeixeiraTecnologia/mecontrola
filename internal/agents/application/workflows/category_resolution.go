package workflows

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

type categorySearcher interface {
	SearchDictionary(ctx context.Context, term, kind string) (interfaces.CategorySearchResult, error)
	ResolveForWrite(ctx context.Context, input interfaces.CategoryWriteRequest) (interfaces.CategoryWriteDecision, error)
}

func SearchAndEnrichCandidates(ctx context.Context, reader categorySearcher, text string, kind interfaces.CategoryKind, expectedVersion int64) ([]PendingCategoryCandidate, error) {
	result, err := reader.SearchDictionary(ctx, text, kind.String())
	if err != nil {
		return nil, fmt.Errorf("category_resolution: search dictionary: %w", err)
	}
	return EnrichCandidatesFromSearch(ctx, reader, result, kind, expectedVersion)
}

func EnrichCandidatesFromSearch(ctx context.Context, reader categorySearcher, result interfaces.CategorySearchResult, kind interfaces.CategoryKind, expectedVersion int64) ([]PendingCategoryCandidate, error) {
	candidates := make([]PendingCategoryCandidate, 0, len(result.Candidates))
	for _, c := range result.Candidates {
		if c.RootCategoryID == (uuid.UUID{}) || c.CategoryID == (uuid.UUID{}) {
			continue
		}
		if c.CategoryID == c.RootCategoryID {
			continue
		}
		decision, err := reader.ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
			RootCategoryID:  c.RootCategoryID,
			SubcategoryID:   c.CategoryID,
			Kind:            kind,
			ExpectedVersion: expectedVersion,
		})
		if err != nil {
			continue
		}
		if decision.SubcategoryID == (uuid.UUID{}) || decision.SubcategoryID == decision.RootCategoryID {
			continue
		}
		candidates = append(candidates, PendingCategoryCandidate{
			RootCategoryID:  decision.RootCategoryID,
			RootSlug:        decision.RootSlug,
			SubcategoryID:   decision.SubcategoryID,
			SubcategorySlug: decision.SubcategorySlug,
			Path:            decision.Path,
			MatchedTerm:     c.MatchedTerm,
			Score:           c.Score,
			Confidence:      c.Confidence,
			MatchQuality:    c.MatchQuality,
			MatchReason:     c.MatchReason,
			SignalType:      c.SignalType,
		})
	}
	return candidates, nil
}

func BuildCandidateListText(candidates []PendingCategoryCandidate) string {
	if len(candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for i, c := range candidates {
		fmt.Fprintf(&b, "%d. %s", i+1, formatCandidatePath(c))
		if i < len(candidates)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func formatCandidatePath(c PendingCategoryCandidate) string {
	if c.Path != "" {
		return c.Path
	}
	if c.RootSlug != "" && c.SubcategorySlug != "" {
		return c.RootSlug + " > " + c.SubcategorySlug
	}
	if c.SubcategorySlug != "" {
		return c.SubcategorySlug
	}
	return c.RootSlug
}
