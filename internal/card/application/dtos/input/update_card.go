package input

import (
	"errors"

	"github.com/google/uuid"
)

type UpdateCard struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Nickname *string
	Bank     *string
	DueDay   *int
}

func (i *UpdateCard) Validate() error {
	var errs []error
	if i.ID == uuid.Nil {
		errs = append(errs, ErrCardIDRequired)
	}
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	if i.DueDay != nil && (*i.DueDay < 1 || *i.DueDay > 31) {
		errs = append(errs, ErrCardDueDayInvalid)
	}
	return errors.Join(errs...)
}
