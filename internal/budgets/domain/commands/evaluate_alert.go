package commands

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type EvaluateAlertCommand struct {
	UserID     uuid.UUID
	Competence valueobjects.Competence
	NowUTC     time.Time
}

func NewEvaluateAlertCommand(
	userID string,
	competence string,
	now time.Time,
) (EvaluateAlertCommand, error) {
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
		return EvaluateAlertCommand{}, errors.Join(errs...)
	}

	return EvaluateAlertCommand{
		UserID:     parsedUserID,
		Competence: parsedCompetence,
		NowUTC:     now.UTC(),
	}, nil
}
