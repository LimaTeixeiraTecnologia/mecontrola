package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type DictionaryEntry struct {
	ID           uuid.UUID
	CategoryID   uuid.UUID
	Kind         valueobjects.Kind
	Term         string
	SignalType   valueobjects.SignalType
	Confidence   valueobjects.Confidence
	IsAmbiguous  bool
	DeprecatedAt *time.Time
}
