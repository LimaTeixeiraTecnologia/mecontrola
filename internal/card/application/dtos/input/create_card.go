package input

import (
	"errors"

	"github.com/google/uuid"
)

type CreateCard struct {
	UserID     uuid.UUID
	Name       string
	Nickname   string
	ClosingDay int
	DueDay     *int
	LimitCents int64
}

func (i *CreateCard) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	if i.Name == "" {
		errs = append(errs, ErrCardNameRequired)
	}
	if i.ClosingDay < 1 || i.ClosingDay > 31 {
		errs = append(errs, ErrCardClosingDayInvalid)
	}
	if i.DueDay != nil && (*i.DueDay < 1 || *i.DueDay > 31) {
		errs = append(errs, ErrCardDueDayInvalid)
	}
	if i.LimitCents < 0 {
		errs = append(errs, ErrCardLimitCentsInvalid)
	}
	return errors.Join(errs...)
}
