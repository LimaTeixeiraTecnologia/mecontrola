package binding

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/events"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SubscriptionBindingService struct {
	identityGateway    appinterfaces.IdentityGateway
	subscriptionBinder appinterfaces.SubscriptionBinder
	publisher          outbox.Publisher
	idGen              id.Generator
}

func NewSubscriptionBindingService(
	identityGateway appinterfaces.IdentityGateway,
	subscriptionBinder appinterfaces.SubscriptionBinder,
	publisher outbox.Publisher,
	idGen id.Generator,
) *SubscriptionBindingService {
	return &SubscriptionBindingService{
		identityGateway:    identityGateway,
		subscriptionBinder: subscriptionBinder,
		publisher:          publisher,
		idGen:              idGen,
	}
}

func (s *SubscriptionBindingService) BindAndConsume(
	ctx context.Context,
	tokenRepo appinterfaces.MagicTokenRepository,
	magicToken entities.MagicToken,
	fromE164 string,
	path valueobjects.ActivationPath,
	now time.Time,
) (entities.MagicToken, error) {
	userResult, err := s.identityGateway.UpsertUserByWhatsApp(ctx, fromE164, magicToken.CustomerEmail())
	if err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding/binding: upsert user: %w", err)
	}

	if err := s.subscriptionBinder.BindUser(ctx, magicToken.SubscriptionID(), userResult.UserID); err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding/binding: bind subscription: %w", err)
	}

	consumed, err := magicToken.MarkConsumed(userResult.UserID, fromE164, path, now)
	if err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding/binding: mark consumed: %w", err)
	}

	if err := tokenRepo.UpdateMarkConsumed(ctx, consumed); err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding/binding: update consumed: %w", err)
	}

	evt, err := events.NewSubscriptionBoundEvent(s.idGen.NewID(), userResult.UserID, consumed, path, now)
	if err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding/binding: %w", err)
	}

	if err := s.publisher.Publish(ctx, evt); err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding/binding: publish event: %w", err)
	}

	return consumed, nil
}
