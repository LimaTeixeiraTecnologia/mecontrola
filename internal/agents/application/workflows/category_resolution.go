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

func SearchAndEnrichCandidates(ctx context.Context, reader categorySearcher, text string, kind interfaces.CategoryKind, expectedVersion int64) ([]PendingCategoryCandidate, int64, error) {
	result, err := reader.SearchDictionary(ctx, text, kind.String())
	if err != nil {
		return nil, 0, fmt.Errorf("category_resolution: search dictionary: %w", err)
	}
	effectiveVersion := expectedVersion
	if effectiveVersion == 0 {
		effectiveVersion = result.Version
	}
	candidates, enrichErr := EnrichCandidatesFromSearch(ctx, reader, result, kind, effectiveVersion)
	return candidates, effectiveVersion, enrichErr
}

func RootCategoryLeafCandidates(ctx context.Context, cats categoryValidator, userID uuid.UUID, text string, kind interfaces.CategoryKind) ([]PendingCategoryCandidate, error) {
	normalized := normalizeCategoryTerm(text)
	if normalized == "" {
		return nil, nil
	}
	roots, err := cats.ListCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("category_resolution: list categories: %w", err)
	}
	for _, root := range roots {
		if root.ParentID != nil {
			continue
		}
		if root.Kind != "" && root.Kind != kind.String() {
			continue
		}
		if normalizeCategoryTerm(root.Name) != normalized && normalizeCategoryTerm(root.Slug) != normalized {
			continue
		}
		if candidates := rootLeafCandidates(root); len(candidates) > 0 {
			return candidates, nil
		}
	}
	return nil, nil
}

func RootLeafCandidatesByRootID(ctx context.Context, cats categoryValidator, userID uuid.UUID, rootID uuid.UUID, kind interfaces.CategoryKind) ([]PendingCategoryCandidate, error) {
	roots, err := cats.ListCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("category_resolution: list categories: %w", err)
	}
	for _, root := range roots {
		if root.ParentID != nil || root.ID != rootID {
			continue
		}
		if root.Kind != "" && root.Kind != kind.String() {
			continue
		}
		return rootLeafCandidates(root), nil
	}
	return nil, nil
}

func rootLeafCandidates(root interfaces.Category) []PendingCategoryCandidate {
	candidates := make([]PendingCategoryCandidate, 0, len(root.Subcategories))
	for _, leaf := range root.Subcategories {
		if leaf.ID == uuid.Nil || leaf.ID == root.ID {
			continue
		}
		candidates = append(candidates, PendingCategoryCandidate{
			RootCategoryID:  root.ID,
			RootSlug:        root.Slug,
			SubcategoryID:   leaf.ID,
			SubcategorySlug: leaf.Slug,
			Path:            root.Name + " > " + leaf.Name,
			Score:           1.0,
			Confidence:      "manual_confirmed",
			MatchQuality:    "manual_canonical",
			SignalType:      "manual_canonical",
			MatchedTerm:     leaf.Slug,
			MatchReason:     "manual canonical id validated",
		})
	}
	return candidates
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
