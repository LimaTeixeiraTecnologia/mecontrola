package input

import (
	"errors"

	"github.com/google/uuid"
)

type AllocationInput struct {
	RootSlug    string
	BasisPoints int
}

type CreateBudgetInput struct {
	UserID      string
	Competence  string
	TotalCents  int64
	Allocations []AllocationInput
}

func (i *CreateBudgetInput) Validate() error {
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
	if len(i.Allocations) == 0 {
		errs = append(errs, ErrInputAllocationsEmpty)
	}
	return errors.Join(errs...)
}
