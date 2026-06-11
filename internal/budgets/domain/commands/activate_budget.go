package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ActivateBudgetCommand struct {
	UserID     uuid.UUID
	Competence valueobjects.Competence
}

func NewActivateBudgetCommand(userID string, competence string) (ActivateBudgetCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedCompetence, err := valueobjects.NewCompetence(competence)
	if err != nil {
		errs = append(errs, ErrCommandInvalidCompetence)
	}

	if len(errs) > 0 {
		return ActivateBudgetCommand{}, errors.Join(errs...)
	}

	return ActivateBudgetCommand{
		UserID:     parsedUserID,
		Competence: parsedCompetence,
	}, nil
}
