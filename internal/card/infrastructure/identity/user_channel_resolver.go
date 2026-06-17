package identity

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
)

type ResolveFunc func(ctx context.Context, userID uuid.UUID) (channel, externalID string, ok bool, err error)

type UserChannelResolverAdapter struct {
	resolve ResolveFunc
}

func NewUserChannelResolverAdapter(resolve ResolveFunc) *UserChannelResolverAdapter {
	return &UserChannelResolverAdapter{resolve: resolve}
}

func (a *UserChannelResolverAdapter) ResolvePreferred(ctx context.Context, userID uuid.UUID) (cardinterfaces.UserChannelPreference, bool, error) {
	if a.resolve == nil {
		return cardinterfaces.UserChannelPreference{}, false, nil
	}
	channel, externalID, ok, err := a.resolve(ctx, userID)
	if err != nil {
		return cardinterfaces.UserChannelPreference{}, false, fmt.Errorf("card/identity: resolve preferred channel: %w", err)
	}
	if !ok {
		return cardinterfaces.UserChannelPreference{}, false, nil
	}
	return cardinterfaces.UserChannelPreference{
		Channel:    channel,
		ExternalID: externalID,
	}, true, nil
}
