package input

import (
	"errors"

	"github.com/google/uuid"
)

type CountCards struct {
	UserID uuid.UUID
}

func (i *CountCards) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	return errors.Join(errs...)
}
