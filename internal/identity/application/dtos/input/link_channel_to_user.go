package input

import (
	"errors"

	"github.com/google/uuid"
)

type LinkChannelToUser struct {
	UserID     uuid.UUID
	Channel    string
	ExternalID string
}

func (i *LinkChannelToUser) Validate() error {
	var errs []error
	if i.UserID == uuid.Nil {
		errs = append(errs, ErrUserIDRequired)
	}
	if i.Channel == "" {
		errs = append(errs, ErrChannelRequired)
	}
	if i.ExternalID == "" {
		errs = append(errs, ErrExternalIDRequired)
	}
	return errors.Join(errs...)
}
