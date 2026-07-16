package input

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type UpsertExpenseInput struct {
	UserID                string
	Source                string
	ExternalTransactionID string
	SubcategoryID         string
	Competence            string
	AmountCents           int64
	OccurredAt            time.Time
	ExpectedVersion       *int64
	Reconcile             bool
}

func (i *UpsertExpenseInput) Validate() error {
	var errs []error
	if _, err := uuid.Parse(i.UserID); err != nil {
		errs = append(errs, ErrInputInvalidUserID)
	}
	if i.Source == "" {
		errs = append(errs, ErrInputInvalidSource)
	}
	if i.ExternalTransactionID == "" {
		errs = append(errs, ErrInputInvalidExternalID)
	}
	if _, err := uuid.Parse(i.SubcategoryID); err != nil {
		errs = append(errs, ErrInputInvalidSubcategory)
	}
	if i.Competence == "" {
		errs = append(errs, ErrInputInvalidCompetence)
	}
	if i.AmountCents <= 0 {
		errs = append(errs, ErrInputAmountCentsInvalid)
	}
	if i.ExpectedVersion != nil && *i.ExpectedVersion <= 0 {
		errs = append(errs, ErrInputExpectedVersion)
	}
	return errors.Join(errs...)
}
