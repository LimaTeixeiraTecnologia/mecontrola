package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

const (
	minRecurrenceMonths = 1
	maxRecurrenceMonths = 12
)

type CreateRecurrenceCommand struct {
	UserID           uuid.UUID
	SourceCompetence valueobjects.Competence
	Months           int
}

func NewCreateRecurrenceCommand(
	userID string,
	sourceCompetence string,
	months int,
) (CreateRecurrenceCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedCompetence, err := valueobjects.NewCompetence(sourceCompetence)
	if err != nil {
		errs = append(errs, ErrCommandInvalidCompetence)
	}

	if months < minRecurrenceMonths || months > maxRecurrenceMonths {
		errs = append(errs, ErrCommandInvalidMonths)
	}

	if len(errs) > 0 {
		return CreateRecurrenceCommand{}, errors.Join(errs...)
	}

	return CreateRecurrenceCommand{
		UserID:           parsedUserID,
		SourceCompetence: parsedCompetence,
		Months:           months,
	}, nil
}
