package commands

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type UpsertExpenseCommand struct {
	UserID          uuid.UUID
	SubcategoryID   uuid.UUID
	Source          valueobjects.ProducerSource
	ExtID           valueobjects.ExternalTransactionID
	Competence      valueobjects.Competence
	AmountCents     int64
	OccurredAt      time.Time
	ExpectedVersion *int64
	Reconcile       bool
}

func NewUpsertExpenseCommand(
	userID string,
	subcategoryID string,
	source string,
	extID string,
	competence string,
	amountCents int64,
	occurredAt time.Time,
	expectedVersion *int64,
	reconcile bool,
) (UpsertExpenseCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedSubID, err := uuid.Parse(subcategoryID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidSubcategory)
	}

	parsedSource, err := valueobjects.NewProducerSource(source)
	if err != nil {
		errs = append(errs, ErrCommandInvalidSource)
	}

	parsedExtID, err := valueobjects.NewExternalTransactionID(extID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidExternalID)
	}

	parsedCompetence, err := valueobjects.NewCompetence(competence)
	if err != nil {
		errs = append(errs, ErrCommandInvalidCompetence)
	}

	if amountCents <= 0 {
		errs = append(errs, ErrCommandInvalidAmount)
	}

	if expectedVersion != nil && *expectedVersion <= 0 {
		errs = append(errs, ErrCommandVersionRequired)
	}

	if len(errs) > 0 {
		return UpsertExpenseCommand{}, errors.Join(errs...)
	}

	return UpsertExpenseCommand{
		UserID:          parsedUserID,
		SubcategoryID:   parsedSubID,
		Source:          parsedSource,
		ExtID:           parsedExtID,
		Competence:      parsedCompetence,
		AmountCents:     amountCents,
		OccurredAt:      occurredAt,
		ExpectedVersion: expectedVersion,
		Reconcile:       reconcile,
	}, nil
}
