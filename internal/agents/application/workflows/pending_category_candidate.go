package workflows

import "github.com/google/uuid"

type PendingCategoryCandidate struct {
	RootCategoryID  uuid.UUID `json:"rootCategoryId"`
	RootSlug        string    `json:"rootSlug"`
	SubcategoryID   uuid.UUID `json:"subcategoryId"`
	SubcategorySlug string    `json:"subcategorySlug"`
	Path            string    `json:"path"`
	MatchedTerm     string    `json:"matchedTerm"`
	Score           float64   `json:"score"`
	Confidence      string    `json:"confidence"`
	MatchQuality    string    `json:"matchQuality"`
	MatchReason     string    `json:"matchReason"`
	SignalType      string    `json:"signalType"`
}
