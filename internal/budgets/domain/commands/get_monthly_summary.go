package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type GetMonthlySummaryCommand struct {
	UserID     uuid.UUID
	Competence valueobjects.Competence
}

func NewGetMonthlySummaryCommand(userID string, competence string) (GetMonthlySummaryCommand, error) {
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
		return GetMonthlySummaryCommand{}, errors.Join(errs...)
	}

	return GetMonthlySummaryCommand{
		UserID:     parsedUserID,
		Competence: parsedCompetence,
	}, nil
}
