package input

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ListAlertsInput struct {
	UserID     string
	Competence *valueobjects.Competence
	RootSlug   *valueobjects.RootSlug
	Threshold  *valueobjects.Threshold
	Cursor     string
	Limit      int
}

func (i *ListAlertsInput) Validate() error {
	var errs []error
	if _, err := uuid.Parse(i.UserID); err != nil {
		errs = append(errs, ErrInputInvalidUserID)
	}
	return errors.Join(errs...)
}
