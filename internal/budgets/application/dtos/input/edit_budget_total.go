package input

import (
	"errors"

	"github.com/google/uuid"
)

type EditBudgetTotalInput struct {
	UserID     string
	Competence string
	TotalCents int64
}

func (i *EditBudgetTotalInput) Validate() error {
	var errs []error
	if _, err := uuid.Parse(i.UserID); err != nil {
		errs = append(errs, ErrInputInvalidUserID)
	}
	if i.Competence == "" {
		errs = append(errs, ErrInputInvalidCompetence)
	}
	if i.TotalCents <= 0 {
		errs = append(errs, ErrInputInvalidTotalCents)
	}
	return errors.Join(errs...)
}
