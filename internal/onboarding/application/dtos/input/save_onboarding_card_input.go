package input

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrCardUserIDRequired   = errors.New("user_id: obrigatório")
	ErrCardNicknameRequired = errors.New("nickname: obrigatório")
	ErrCardClosingDayRange  = errors.New("closing_day: deve estar entre 1 e 31")
)

type SaveOnboardingCardInput struct {
	UserID     uuid.UUID
	Nickname   string
	ClosingDay int
}

func (i *SaveOnboardingCardInput) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	if i.Nickname == "" {
		errs = append(errs, ErrCardNicknameRequired)
	}
	if i.ClosingDay < 1 || i.ClosingDay > 31 {
		errs = append(errs, ErrCardClosingDayRange)
	}
	return errors.Join(errs...)
}
