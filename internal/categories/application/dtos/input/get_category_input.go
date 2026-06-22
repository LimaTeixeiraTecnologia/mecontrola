package input

import (
	"errors"

	"github.com/google/uuid"
)

type GetCategoryInput struct {
	ID                uuid.UUID
	IncludeDeprecated bool
}

func (i *GetCategoryInput) Validate() error {
	var errs []error
	if i.ID == uuid.Nil {
		errs = append(errs, ErrCategoryIDRequired)
	}
	return errors.Join(errs...)
}
