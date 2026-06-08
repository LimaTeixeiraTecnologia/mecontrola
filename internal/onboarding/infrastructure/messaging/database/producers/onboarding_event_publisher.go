package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeSubscriptionBound = "onboarding.subscription_bound"

type SubscriptionBoundPayload struct {
	EventID               string    `json:"event_id"`
	UserID                string    `json:"user_id"`
	SubscriptionID        string    `json:"subscription_id"`
	FunnelTokenHashPrefix string    `json:"funnel_token_hash_prefix"`
	ActivationPath        string    `json:"activation_path"`
	BoundAt               time.Time `json:"bound_at"`
}

type OnboardingEventPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
}

func NewOnboardingEventPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
) *OnboardingEventPublisher {
	return &OnboardingEventPublisher{outboxFactory: outboxFactory, cfg: cfg, idGen: idGen}
}

func (p *OnboardingEventPublisher) PublishSubscriptionBound(
	ctx context.Context,
	tx database.DBTX,
	payload SubscriptionBoundPayload,
) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("onboarding/producer: marshal payload: %w", err)
	}

	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:            p.idGen.NewID(),
		Type:          eventTypeSubscriptionBound,
		AggregateType: "onboarding_token",
		AggregateID:   payload.UserID,
		Payload:       raw,
		OccurredAt:    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("onboarding/producer: new event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(tx)
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)

	if err := publisher.Publish(ctx, evt); err != nil {
		return fmt.Errorf("onboarding/producer: publish: %w", err)
	}
	return nil
}
