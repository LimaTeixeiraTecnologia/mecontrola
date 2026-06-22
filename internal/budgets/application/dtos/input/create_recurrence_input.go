package input

import (
	"errors"

	"github.com/google/uuid"
)

type CreateRecurrenceInput struct {
	UserID           string
	SourceCompetence string
	Months           int
}

func (i *CreateRecurrenceInput) Validate() error {
	var errs []error
	if _, err := uuid.Parse(i.UserID); err != nil {
		errs = append(errs, ErrInputInvalidUserID)
	}
	if i.SourceCompetence == "" {
		errs = append(errs, ErrInputInvalidCompetence)
	}
	if i.Months < 1 || i.Months > 12 {
		errs = append(errs, ErrInputMonthsOutOfRange)
	}
	return errors.Join(errs...)
}
