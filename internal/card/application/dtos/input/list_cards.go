package input

import (
	"errors"

	"github.com/google/uuid"
)

type ListCards struct {
	UserID uuid.UUID
	Cursor string
	Limit  int
}

func (i *ListCards) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	return errors.Join(errs...)
}
