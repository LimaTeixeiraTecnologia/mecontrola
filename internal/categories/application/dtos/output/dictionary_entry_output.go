package output

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
)

type DictionaryEntryOutput struct {
	ID           uuid.UUID `json:"id"`
	CategoryID   uuid.UUID `json:"category_id"`
	Kind         string    `json:"kind"`
	Term         string    `json:"term"`
	SignalType   string    `json:"signal_type"`
	Confidence   string    `json:"confidence"`
	IsAmbiguous  bool      `json:"is_ambiguous"`
	DeprecatedAt *string   `json:"deprecated_at,omitempty"`
}

type ListDictionaryOutput struct {
	Entries    []DictionaryEntryOutput `json:"entries"`
	NextCursor string                  `json:"next_cursor,omitempty"`
	Version    int64                   `json:"version"`
}

func NewDictionaryEntryOutputFromEntity(e entities.DictionaryEntry) DictionaryEntryOutput {
	out := DictionaryEntryOutput{
		ID:          e.ID,
		CategoryID:  e.CategoryID,
		Kind:        e.Kind.String(),
		Term:        e.Term,
		SignalType:  e.SignalType.String(),
		Confidence:  e.Confidence.String(),
		IsAmbiguous: e.IsAmbiguous,
	}
	if e.DeprecatedAt != nil {
		ts := e.DeprecatedAt.Format("2006-01-02T15:04:05Z")
		out.DeprecatedAt = &ts
	}
	return out
}
