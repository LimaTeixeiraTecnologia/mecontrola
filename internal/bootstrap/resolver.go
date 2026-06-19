package bootstrap

import (
	"context"

	"github.com/google/uuid"

	budgetsidentity "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/identity"
	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	cardidentity "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
)

func BuildBudgetsChannelResolver(identityModule identity.IdentityModule) *budgetsidentity.UserChannelResolverAdapter {
	if identityModule.ResolvePreferredChannel == nil {
		return nil
	}
	return budgetsidentity.NewUserChannelResolverAdapter(func(ctx context.Context, userID uuid.UUID) (string, string, bool, error) {
		result, ok, err := identityModule.ResolvePreferredChannel.Execute(ctx, userID)
		if err != nil {
			return "", "", false, err
		}
		if !ok {
			return "", "", false, nil
		}
		return result.Channel, result.ExternalID, true, nil
	})
}

func BuildCardChannelResolver(identityModule identity.IdentityModule) cardinterfaces.UserChannelResolver {
	if identityModule.ResolvePreferredChannel == nil {
		return nil
	}
	return cardidentity.NewUserChannelResolverAdapter(func(ctx context.Context, userID uuid.UUID) (string, string, bool, error) {
		result, ok, err := identityModule.ResolvePreferredChannel.Execute(ctx, userID)
		if err != nil {
			return "", "", false, err
		}
		if !ok {
			return "", "", false, nil
		}
		return result.Channel, result.ExternalID, true, nil
	})
}
