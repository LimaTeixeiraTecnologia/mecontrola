package services

import (
	"cmp"
	"slices"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type Candidate struct {
	CategoryID     uuid.UUID
	RootCategoryID uuid.UUID
	Path           string
	MatchedTerm    string
	SignalType     valueobjects.SignalType
	Confidence     valueobjects.Confidence
	IsAmbiguous    bool
	MatchReason    string
}

func (c Candidate) GetCategoryID() uuid.UUID     { return c.CategoryID }
func (c Candidate) GetRootCategoryID() uuid.UUID { return c.RootCategoryID }
func (c Candidate) GetPath() string              { return c.Path }
func (c Candidate) GetMatchedTerm() string       { return c.MatchedTerm }
func (c Candidate) GetSignalType() valueobjects.SignalType {
	return c.SignalType
}
func (c Candidate) GetConfidence() valueobjects.Confidence { return c.Confidence }
func (c Candidate) GetIsAmbiguous() bool                   { return c.IsAmbiguous }
func (c Candidate) GetMatchReason() string                 { return c.MatchReason }

type CandidateResolver struct{}

func NewCandidateResolver() *CandidateResolver {
	return &CandidateResolver{}
}

func (r *CandidateResolver) Resolve(entries []entities.DictionaryEntry, categories map[uuid.UUID]entities.Category) ([]Candidate, bool) {
	if len(entries) == 0 {
		return nil, false
	}

	grouped := make(map[uuid.UUID][]entities.DictionaryEntry)
	for _, entry := range entries {
		grouped[entry.CategoryID] = append(grouped[entry.CategoryID], entry)
	}

	candidates := make([]Candidate, 0, len(grouped))
	for categoryID, group := range grouped {
		winner := r.selectWinner(group)
		category := categories[categoryID]

		candidate := Candidate{
			CategoryID:     categoryID,
			RootCategoryID: r.findRootID(category, categories),
			Path:           r.buildPath(category, categories),
			MatchedTerm:    winner.Term,
			SignalType:     winner.SignalType,
			Confidence:     winner.Confidence,
			IsAmbiguous:    winner.IsAmbiguous,
			MatchReason:    r.buildMatchReason(winner),
		}
		candidates = append(candidates, candidate)
	}

	r.sortCandidates(candidates)

	hasMore := len(candidates) > 3
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}

	if len(candidates) > 1 {
		for i := range candidates {
			candidates[i].IsAmbiguous = true
		}
	}

	return candidates, hasMore
}

func (r *CandidateResolver) selectWinner(entries []entities.DictionaryEntry) entities.DictionaryEntry {
	if len(entries) == 1 {
		return entries[0]
	}

	slices.SortFunc(entries, func(a, b entities.DictionaryEntry) int {
		return cmp.Compare(b.SignalType.Precedence(), a.SignalType.Precedence())
	})

	return entries[0]
}

func (r *CandidateResolver) sortCandidates(candidates []Candidate) {
	slices.SortFunc(candidates, func(a, b Candidate) int {
		precDiff := cmp.Compare(b.SignalType.Precedence(), a.SignalType.Precedence())
		if precDiff != 0 {
			return precDiff
		}
		return cmp.Compare(a.Path, b.Path)
	})
}

func (r *CandidateResolver) findRootID(category entities.Category, categories map[uuid.UUID]entities.Category) uuid.UUID {
	if category.IsRoot() {
		return category.ID
	}
	if category.ParentID != nil {
		parent := categories[*category.ParentID]
		if parent.IsRoot() {
			return parent.ID
		}
	}
	return category.ID
}

func (r *CandidateResolver) buildPath(category entities.Category, categories map[uuid.UUID]entities.Category) string {
	if category.IsRoot() {
		return category.Name
	}

	if category.ParentID != nil {
		parent := categories[*category.ParentID]
		return parent.Name + " > " + category.Name
	}
	return category.Name
}

func (r *CandidateResolver) buildMatchReason(entry entities.DictionaryEntry) string {
	switch entry.SignalType {
	case valueobjects.SignalTypeCanonicalName:
		return "canonical name"
	case valueobjects.SignalTypeAlias:
		return "alias inequívoco"
	case valueobjects.SignalTypePhrase:
		return "phrase match"
	case valueobjects.SignalTypeMerchant:
		return "merchant match"
	case valueobjects.SignalTypeSegment:
		return "segment match"
	default:
		return "match"
	}
}
