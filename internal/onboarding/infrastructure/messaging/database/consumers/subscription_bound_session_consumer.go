package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type startBudgetConfigurator interface {
	Execute(ctx context.Context, in usecases.StartBudgetConfigurationInput) (usecases.StartBudgetConfigurationResult, error)
}

type subscriptionBoundPayload struct {
	UserID string `json:"user_id"`
}

type SubscriptionBoundSessionConsumer struct {
	usecase     startBudgetConfigurator
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewSubscriptionBoundSessionConsumer(
	uc startBudgetConfigurator,
	o11y observability.Observability,
) *SubscriptionBoundSessionConsumer {
	decodeFails := o11y.Metrics().Counter(
		"onboarding_subscription_bound_session_consumer_decode_failed_total",
		"Total de falhas de decode do consumer subscription_bound_session",
		"1",
	)
	return &SubscriptionBoundSessionConsumer{
		usecase:     uc,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *SubscriptionBoundSessionConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.consumer.subscription_bound_session.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("onboarding.consumer.subscription_bound_session: unexpected payload type %T", event.GetPayload())
	}

	var p subscriptionBoundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("onboarding.consumer.subscription_bound_session: unmarshal payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("onboarding.consumer.subscription_bound_session: user_id invalido: %w", err)
	}

	if _, err := c.usecase.Execute(ctx, usecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: entities.OnboardingChannelWhatsApp,
	}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.subscription_bound_session: start budget: %w", err)
	}

	return nil
}
