package input

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

type EditCategoryPercentageInput struct {
	UserID     string
	Competence string
	RootSlug   string
	Percentage int
}

func (i *EditCategoryPercentageInput) Validate() error {
	var errs []error
	if _, err := uuid.Parse(i.UserID); err != nil {
		errs = append(errs, ErrInputInvalidUserID)
	}
	if i.Competence == "" {
		errs = append(errs, ErrInputInvalidCompetence)
	}
	if strings.TrimSpace(i.RootSlug) == "" {
		errs = append(errs, ErrInputInvalidRootSlug)
	}
	if i.Percentage < 0 || i.Percentage > 100 {
		errs = append(errs, ErrInputPercentageRange)
	}
	return errors.Join(errs...)
}
