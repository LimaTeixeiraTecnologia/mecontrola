package input

import (
	"errors"

	"github.com/google/uuid"
)

type GetCard struct {
	ID     uuid.UUID
	UserID uuid.UUID
}

func (i *GetCard) Validate() error {
	var errs []error
	if i.ID == uuid.Nil {
		errs = append(errs, ErrCardIDRequired)
	}
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	return errors.Join(errs...)
}
