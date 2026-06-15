package input

import "github.com/google/uuid"

type LinkChannelToUser struct {
	UserID     uuid.UUID
	Channel    string
	ExternalID string
}
