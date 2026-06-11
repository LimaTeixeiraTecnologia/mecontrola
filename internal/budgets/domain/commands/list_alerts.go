package commands

import (
	"errors"

	"github.com/google/uuid"
)

const (
	defaultAlertLimit = 50
	maxAlertLimit     = 200
)

type ListAlertsCommand struct {
	UserID uuid.UUID
	Cursor string
	Limit  int
}

func NewListAlertsCommand(userID string, cursor string, limit int) (ListAlertsCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	if limit < 0 {
		errs = append(errs, ErrCommandInvalidLimit)
	}

	if len(errs) > 0 {
		return ListAlertsCommand{}, errors.Join(errs...)
	}

	normalized := limit
	if normalized <= 0 {
		normalized = defaultAlertLimit
	}
	if normalized > maxAlertLimit {
		normalized = maxAlertLimit
	}

	return ListAlertsCommand{
		UserID: parsedUserID,
		Cursor: cursor,
		Limit:  normalized,
	}, nil
}
