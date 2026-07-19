package workflows

import (
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

type CategoryCatalogEntry struct {
	RootID   uuid.UUID
	RootName string
	RootSlug string
	LeafID   uuid.UUID
	LeafName string
	LeafSlug string
}

func (e CategoryCatalogEntry) Path() string {
	return e.RootName + " > " + e.LeafName
}

type UserCategoryAction int

const (
	UserCategoryActionMatchedLeaf UserCategoryAction = iota + 1
	UserCategoryActionMatchedRoot
	UserCategoryActionMatchedMany
	UserCategoryActionNoMatch
)

type UserCategoryMatch struct {
	Action UserCategoryAction
	Leaf   CategoryCatalogEntry
	RootID uuid.UUID
	Leaves []CategoryCatalogEntry
}

var categoryTextSeparators = []string{" > ", ">", " e ", ","}

func DecideUserCategoryText(catalog []CategoryCatalogEntry, text string) UserCategoryMatch {
	normalized := normalizeCategoryTerm(text)
	if normalized == "" || len(catalog) == 0 {
		return UserCategoryMatch{Action: UserCategoryActionNoMatch}
	}

	if match := matchLeafTerm(catalog, normalized); match.Action != UserCategoryActionNoMatch {
		return match
	}

	if rootID, ok := matchRootTerm(catalog, normalized); ok {
		return UserCategoryMatch{Action: UserCategoryActionMatchedRoot, RootID: rootID, Leaves: leavesOfRoot(catalog, rootID)}
	}

	if first, last, ok := splitCategoryTerm(normalized); ok {
		if rootID, rootOK := matchRootTerm(catalog, first); rootOK {
			rootLeaves := leavesOfRoot(catalog, rootID)
			if leafMatch := matchLeafTerm(rootLeaves, last); leafMatch.Action == UserCategoryActionMatchedLeaf {
				return leafMatch
			}
			return UserCategoryMatch{Action: UserCategoryActionMatchedRoot, RootID: rootID, Leaves: rootLeaves}
		}
	}

	return UserCategoryMatch{Action: UserCategoryActionNoMatch}
}

func matchLeafTerm(catalog []CategoryCatalogEntry, normalized string) UserCategoryMatch {
	matches := make([]CategoryCatalogEntry, 0, 2)
	for _, entry := range catalog {
		if categoryTermEquals(entry, normalized) {
			matches = append(matches, entry)
		}
	}
	switch len(matches) {
	case 0:
		return UserCategoryMatch{Action: UserCategoryActionNoMatch}
	case 1:
		return UserCategoryMatch{Action: UserCategoryActionMatchedLeaf, Leaf: matches[0]}
	default:
		return UserCategoryMatch{Action: UserCategoryActionMatchedMany, Leaves: matches}
	}
}

func categoryTermEquals(entry CategoryCatalogEntry, normalized string) bool {
	candidates := []string{
		entry.LeafName,
		entry.LeafSlug,
		entry.Path(),
		entry.RootName + " e " + entry.LeafName,
	}
	for _, c := range candidates {
		if normalizeCategoryTerm(c) == normalized {
			return true
		}
	}
	return false
}

func matchRootTerm(catalog []CategoryCatalogEntry, normalized string) (uuid.UUID, bool) {
	for _, entry := range catalog {
		if normalizeCategoryTerm(entry.RootName) == normalized || normalizeCategoryTerm(entry.RootSlug) == normalized {
			return entry.RootID, true
		}
	}
	return uuid.Nil, false
}

func leavesOfRoot(catalog []CategoryCatalogEntry, rootID uuid.UUID) []CategoryCatalogEntry {
	leaves := make([]CategoryCatalogEntry, 0, len(catalog))
	for _, entry := range catalog {
		if entry.RootID == rootID {
			leaves = append(leaves, entry)
		}
	}
	return leaves
}

func splitCategoryTerm(normalized string) (string, string, bool) {
	for _, sep := range categoryTextSeparators {
		if idx := strings.Index(normalized, sep); idx > 0 {
			first := strings.TrimSpace(normalized[:idx])
			last := strings.TrimSpace(normalized[idx+len(sep):])
			if first != "" && last != "" {
				return first, last, true
			}
		}
	}
	return "", "", false
}

func normalizeCategoryTerm(s string) string {
	tokens := strings.Fields(normalizeText(s))
	for i, token := range tokens {
		tokens[i] = singularizeToken(token)
	}
	return strings.Join(tokens, " ")
}

func singularizeToken(token string) string {
	if len(token) <= 3 {
		return token
	}
	switch {
	case strings.HasSuffix(token, "oes"):
		return token[:len(token)-3] + "ao"
	case strings.HasSuffix(token, "aes"):
		return token[:len(token)-3] + "ao"
	case strings.HasSuffix(token, "ais"):
		return token[:len(token)-3] + "al"
	case strings.HasSuffix(token, "eis"):
		return token[:len(token)-3] + "el"
	case strings.HasSuffix(token, "ois"):
		return token[:len(token)-3] + "ol"
	case strings.HasSuffix(token, "ns"):
		return token[:len(token)-2] + "m"
	case strings.HasSuffix(token, "res"), strings.HasSuffix(token, "ses"), strings.HasSuffix(token, "zes"):
		return token[:len(token)-2]
	case strings.HasSuffix(token, "s"):
		return token[:len(token)-1]
	default:
		return token
	}
}

func FlattenCategoryCatalog(roots []interfaces.Category, kind interfaces.CategoryKind) []CategoryCatalogEntry {
	entries := make([]CategoryCatalogEntry, 0, len(roots)*8)
	for _, root := range roots {
		if root.ParentID != nil {
			continue
		}
		if root.Kind != "" && root.Kind != kind.String() {
			continue
		}
		for _, leaf := range root.Subcategories {
			if leaf.ID == uuid.Nil || leaf.ID == root.ID {
				continue
			}
			entries = append(entries, CategoryCatalogEntry{
				RootID:   root.ID,
				RootName: root.Name,
				RootSlug: root.Slug,
				LeafID:   leaf.ID,
				LeafName: leaf.Name,
				LeafSlug: leaf.Slug,
			})
		}
	}
	return entries
}

func BuildRootOnlyCandidates(roots []interfaces.Category, kind interfaces.CategoryKind) []PendingCategoryCandidate {
	candidates := make([]PendingCategoryCandidate, 0, len(roots))
	for _, root := range roots {
		if root.ParentID != nil {
			continue
		}
		if root.Kind != "" && root.Kind != kind.String() {
			continue
		}
		candidates = append(candidates, PendingCategoryCandidate{
			RootCategoryID: root.ID,
			RootSlug:       root.Slug,
			Path:           root.Name,
			Score:          1.0,
			Confidence:     "manual_confirmed",
			MatchQuality:   "manual_canonical",
			SignalType:     "manual_canonical",
			MatchedTerm:    root.Slug,
			MatchReason:    "root listing offered",
		})
	}
	return candidates
}

func CatalogEntryToCandidate(entry CategoryCatalogEntry) PendingCategoryCandidate {
	return PendingCategoryCandidate{
		RootCategoryID:  entry.RootID,
		RootSlug:        entry.RootSlug,
		SubcategoryID:   entry.LeafID,
		SubcategorySlug: entry.LeafSlug,
		Path:            entry.Path(),
		Score:           1.0,
		Confidence:      "manual_confirmed",
		MatchQuality:    "manual_canonical",
		SignalType:      "manual_canonical",
		MatchedTerm:     entry.LeafSlug,
		MatchReason:     "manual canonical id validated",
	}
}
