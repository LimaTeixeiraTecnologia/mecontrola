package input

import (
	"errors"

	"github.com/google/uuid"
)

type ActivateBudgetInput struct {
	UserID     string
	Competence string
}

func (i *ActivateBudgetInput) Validate() error {
	var errs []error
	if _, err := uuid.Parse(i.UserID); err != nil {
		errs = append(errs, ErrInputInvalidUserID)
	}
	if i.Competence == "" {
		errs = append(errs, ErrInputInvalidCompetence)
	}
	return errors.Join(errs...)
}
