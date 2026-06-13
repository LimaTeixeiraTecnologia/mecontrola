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

	parsedSubID, parsedCompetence, optErrs := parseIngestOptionalFields(subcategoryID, competence)
	errs = append(errs, optErrs...)
	errs = append(errs, validateIngestScalars(parsedKind, amountCents, version, occurredAt)...)

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

func parseIngestOptionalFields(subcategoryID, competence string) (uuid.UUID, valueobjects.Competence, []error) {
	var errs []error
	parsedSubID := uuid.Nil
	if subcategoryID != "" {
		id, err := uuid.Parse(subcategoryID)
		if err != nil {
			errs = append(errs, ErrCommandInvalidSubcategory)
		} else {
			parsedSubID = id
		}
	}
	parsedCompetence := valueobjects.Competence{}
	if competence != "" {
		c, err := valueobjects.NewCompetence(competence)
		if err != nil {
			errs = append(errs, ErrCommandInvalidCompetence)
		} else {
			parsedCompetence = c
		}
	}
	return parsedSubID, parsedCompetence, errs
}

func validateIngestScalars(kind valueobjects.MutationKind, amountCents, version int64, occurredAt time.Time) []error {
	var errs []error
	if kind != valueobjects.MutationKindDelete && amountCents <= 0 {
		errs = append(errs, ErrCommandInvalidAmount)
	}
	if occurredAt.IsZero() {
		errs = append(errs, ErrCommandInvalidOccurredAt)
	}
	if kind == valueobjects.MutationKindCreate && version != 1 {
		errs = append(errs, ErrCommandVersionRequired)
	}
	return errs
}
