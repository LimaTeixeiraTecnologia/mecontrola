package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type EditBudgetTotalCommand struct {
	UserID     uuid.UUID
	Competence valueobjects.Competence
	TotalCents int64
}

func NewEditBudgetTotalCommand(userID string, competence string, totalCents int64) (EditBudgetTotalCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedCompetence, err := valueobjects.NewCompetence(competence)
	if err != nil {
		errs = append(errs, ErrCommandInvalidCompetence)
	}

	if totalCents <= 0 {
		errs = append(errs, ErrCommandInvalidTotalCents)
	}

	if len(errs) > 0 {
		return EditBudgetTotalCommand{}, errors.Join(errs...)
	}

	return EditBudgetTotalCommand{
		UserID:     parsedUserID,
		Competence: parsedCompetence,
		TotalCents: totalCents,
	}, nil
}
