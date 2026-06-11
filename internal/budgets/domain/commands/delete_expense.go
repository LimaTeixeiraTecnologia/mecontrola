package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteExpenseCommand struct {
	UserID          uuid.UUID
	Source          valueobjects.ProducerSource
	ExtID           valueobjects.ExternalTransactionID
	ExpectedVersion int64
}

func NewDeleteExpenseCommand(
	userID string,
	source string,
	extID string,
	expectedVersion int64,
) (DeleteExpenseCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedSource, err := valueobjects.NewProducerSource(source)
	if err != nil {
		errs = append(errs, ErrCommandInvalidSource)
	}

	parsedExtID, err := valueobjects.NewExternalTransactionID(extID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidExternalID)
	}

	if len(errs) > 0 {
		return DeleteExpenseCommand{}, errors.Join(errs...)
	}

	return DeleteExpenseCommand{
		UserID:          parsedUserID,
		Source:          parsedSource,
		ExtID:           parsedExtID,
		ExpectedVersion: expectedVersion,
	}, nil
}
