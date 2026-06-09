package output

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CandidateOutput struct {
	CategoryID     uuid.UUID `json:"category_id"`
	RootCategoryID uuid.UUID `json:"root_category_id"`
	Path           string    `json:"path"`
	MatchedTerm    string    `json:"matched_term"`
	SignalType     string    `json:"signal_type"`
	Confidence     string    `json:"confidence"`
	IsAmbiguous    bool      `json:"is_ambiguous"`
	MatchReason    string    `json:"match_reason"`
}

type DictionarySearchOutput struct {
	Result        string            `json:"result"`
	Candidates    []CandidateOutput `json:"candidates,omitempty"`
	HasMore       bool              `json:"has_more"`
	SignalTypeTop string            `json:"-"`
	IsAmbiguous   bool              `json:"-"`
	Version       int64             `json:"version"`
}

func NewCandidateOutputFromService(c CandidateLike) CandidateOutput {
	return CandidateOutput{
		CategoryID:     c.GetCategoryID(),
		RootCategoryID: c.GetRootCategoryID(),
		Path:           c.GetPath(),
		MatchedTerm:    c.GetMatchedTerm(),
		SignalType:     c.GetSignalType().String(),
		Confidence:     c.GetConfidence().String(),
		IsAmbiguous:    c.GetIsAmbiguous(),
		MatchReason:    c.GetMatchReason(),
	}
}

type CandidateLike interface {
	GetCategoryID() uuid.UUID
	GetRootCategoryID() uuid.UUID
	GetPath() string
	GetMatchedTerm() string
	GetSignalType() valueobjects.SignalType
	GetConfidence() valueobjects.Confidence
	GetIsAmbiguous() bool
	GetMatchReason() string
}
