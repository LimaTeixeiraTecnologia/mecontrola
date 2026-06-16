package identity

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
)

type ResolveFunc func(ctx context.Context, userID uuid.UUID) (channel, externalID string, ok bool, err error)

type UserChannelResolverAdapter struct {
	resolve ResolveFunc
}

func NewUserChannelResolverAdapter(resolve ResolveFunc) *UserChannelResolverAdapter {
	return &UserChannelResolverAdapter{resolve: resolve}
}

func (a *UserChannelResolverAdapter) ResolvePreferred(ctx context.Context, userID uuid.UUID) (budgetsinterfaces.UserChannelPreference, bool, error) {
	if a.resolve == nil {
		return budgetsinterfaces.UserChannelPreference{}, false, nil
	}
	channel, externalID, ok, err := a.resolve(ctx, userID)
	if err != nil {
		return budgetsinterfaces.UserChannelPreference{}, false, fmt.Errorf("budgets/identity: resolve preferred channel: %w", err)
	}
	if !ok {
		return budgetsinterfaces.UserChannelPreference{}, false, nil
	}
	return budgetsinterfaces.UserChannelPreference{
		Channel:    channel,
		ExternalID: externalID,
	}, true, nil
}
