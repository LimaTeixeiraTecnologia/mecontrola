package input

import (
	"errors"

	"github.com/google/uuid"
)

type UpdateCard struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       *string
	Nickname   *string
	ClosingDay *int
	DueDay     *int
}

func (i *UpdateCard) Validate() error {
	var errs []error
	if i.ID == uuid.Nil {
		errs = append(errs, ErrCardIDRequired)
	}
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	return errors.Join(errs...)
}
