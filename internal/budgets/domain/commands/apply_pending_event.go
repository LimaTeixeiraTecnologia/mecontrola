package commands

import (
	"github.com/google/uuid"
)

type ApplyPendingEventCommand struct {
	EventID uuid.UUID
}

func NewApplyPendingEventCommand(eventID string) (ApplyPendingEventCommand, error) {
	parsedEventID, err := uuid.Parse(eventID)
	if err != nil {
		return ApplyPendingEventCommand{}, ErrCommandInvalidEventID
	}
	return ApplyPendingEventCommand{EventID: parsedEventID}, nil
}
