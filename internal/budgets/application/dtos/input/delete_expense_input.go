package input

import (
	"errors"

	"github.com/google/uuid"
)

type DeleteExpenseInput struct {
	UserID                string
	Source                string
	ExternalTransactionID string
	ExpectedVersion       int64
}

func (i *DeleteExpenseInput) Validate() error {
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
	return errors.Join(errs...)
}
