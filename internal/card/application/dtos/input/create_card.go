package input

import (
	"errors"

	"github.com/google/uuid"
)

type CreateCard struct {
	UserID   uuid.UUID
	Nickname string
	Bank     string
	DueDay   int
}

func (i *CreateCard) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	if i.Bank == "" {
		errs = append(errs, ErrCardBankRequired)
	}
	if i.DueDay < 1 || i.DueDay > 31 {
		errs = append(errs, ErrCardDueDayInvalid)
	}
	return errors.Join(errs...)
}
