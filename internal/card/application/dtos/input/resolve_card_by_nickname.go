package input

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

type ResolveCardByNickname struct {
	UserID   uuid.UUID
	Nickname string
}

func (i *ResolveCardByNickname) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrCardUserIDRequired)
	}
	if strings.TrimSpace(i.Nickname) == "" {
		errs = append(errs, ErrCardNicknameRequired)
	}
	return errors.Join(errs...)
}
