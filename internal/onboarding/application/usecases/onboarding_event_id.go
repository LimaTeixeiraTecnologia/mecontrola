package usecases

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

func newEventID(idGen id.Generator) uuid.UUID {
	parsed, err := uuid.Parse(idGen.NewID())
	if err != nil {
		return uuid.New()
	}
	return parsed
}
