package commands

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type IngestExternalExpenseCommand struct {
	EventID       uuid.UUID
	UserID        uuid.UUID
	Source        valueobjects.ProducerSource
	ExtID         valueobjects.ExternalTransactionID
	SubcategoryID uuid.UUID
	Competence    valueobjects.Competence
	MutationKind  valueobjects.MutationKind
	Version       int64
	AmountCents   int64
	OccurredAt    time.Time
}

func NewIngestExternalExpenseCommand(
	eventID string,
	userID string,
	source string,
	extID string,
	subcategoryID string,
	competence string,
	operation string,
	version int64,
	amountCents int64,
	occurredAt time.Time,
) (IngestExternalExpenseCommand, error) {
	var errs []error

	parsedEventID, err := uuid.Parse(eventID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidEventID)
	}

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

	parsedKind, err := valueobjects.ParseMutationKind(operation)
	if err != nil {
		errs = append(errs, ErrCommandInvalidMutationKind)
	}

	parsedSubID := uuid.Nil
	if subcategoryID != "" {
		parsedSubID, err = uuid.Parse(subcategoryID)
		if err != nil {
			errs = append(errs, ErrCommandInvalidSubcategory)
		}
	}

	parsedCompetence := valueobjects.Competence{}
	if competence != "" {
		parsedCompetence, err = valueobjects.NewCompetence(competence)
		if err != nil {
			errs = append(errs, ErrCommandInvalidCompetence)
		}
	}

	if parsedKind != valueobjects.MutationKindDelete && amountCents <= 0 {
		errs = append(errs, ErrCommandInvalidAmount)
	}

	if occurredAt.IsZero() {
		errs = append(errs, ErrCommandInvalidOccurredAt)
	}

	if parsedKind == valueobjects.MutationKindCreate && version != 1 {
		errs = append(errs, ErrCommandVersionRequired)
	}

	if len(errs) > 0 {
		return IngestExternalExpenseCommand{}, errors.Join(errs...)
	}

	return IngestExternalExpenseCommand{
		EventID:       parsedEventID,
		UserID:        parsedUserID,
		Source:        parsedSource,
		ExtID:         parsedExtID,
		SubcategoryID: parsedSubID,
		Competence:    parsedCompetence,
		MutationKind:  parsedKind,
		Version:       version,
		AmountCents:   amountCents,
		OccurredAt:    occurredAt,
	}, nil
}
