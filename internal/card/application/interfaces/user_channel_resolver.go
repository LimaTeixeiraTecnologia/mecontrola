package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type UserChannelPreference struct {
	Channel    string
	ExternalID string
}

type UserChannelResolver interface {
	ResolvePreferred(ctx context.Context, userID uuid.UUID) (UserChannelPreference, bool, error)
}
