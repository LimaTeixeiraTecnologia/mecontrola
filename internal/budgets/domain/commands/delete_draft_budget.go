package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteDraftBudgetCommand struct {
	UserID     uuid.UUID
	Competence valueobjects.Competence
}

func NewDeleteDraftBudgetCommand(userID string, competence string) (DeleteDraftBudgetCommand, error) {
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
		return DeleteDraftBudgetCommand{}, errors.Join(errs...)
	}

	return DeleteDraftBudgetCommand{
		UserID:     parsedUserID,
		Competence: parsedCompetence,
	}, nil
}
