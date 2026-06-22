package input

import (
	"errors"

	"github.com/google/uuid"
)

type UpdateCardLimit struct {
	CardID          uuid.UUID
	UserID          uuid.UUID
	LimitCents      int64
	ExpectedVersion *int64
}

func (i *UpdateCardLimit) Validate() error {
	var errs []error
	if i.CardID == uuid.Nil {
		errs = append(errs, ErrCardIDCardRequired)
	}
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	if i.LimitCents <= 0 {
		errs = append(errs, ErrCardLimitCentsInvalid)
	}
	return errors.Join(errs...)
}
