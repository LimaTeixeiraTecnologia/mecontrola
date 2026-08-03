package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrCommandInvalidSourceCompetence = errors.New("budgets.command: source competence inválida")

type SyncFutureBudgetsCommand struct {
	UserID           uuid.UUID
	SourceCompetence valueobjects.Competence
}

func NewSyncFutureBudgetsCommand(userID string, sourceCompetence string) (SyncFutureBudgetsCommand, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return SyncFutureBudgetsCommand{}, ErrCommandInvalidUserID
	}

	parsedCompetence, err := valueobjects.NewCompetence(sourceCompetence)
	if err != nil {
		return SyncFutureBudgetsCommand{}, ErrCommandInvalidSourceCompetence
	}

	return SyncFutureBudgetsCommand{
		UserID:           parsedUserID,
		SourceCompetence: parsedCompetence,
	}, nil
}
