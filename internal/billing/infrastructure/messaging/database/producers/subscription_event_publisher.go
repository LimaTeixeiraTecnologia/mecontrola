package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SubscriptionEventPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
}

func NewSubscriptionEventPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
) *SubscriptionEventPublisher {
	return &SubscriptionEventPublisher{outboxFactory: outboxFactory, cfg: cfg, idGen: idGen}
}

func (p *SubscriptionEventPublisher) PublishActivated(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
	funnelToken string,
	customerMobileE164 string,
	customerEmail string,
	externalSaleID string,
) error {
	payload := SubscriptionActivatedPayload{
		SubscriptionID:     subscriptionID,
		FunnelToken:        funnelToken,
		PlanCode:           string(sub.Plan().Code()),
		ExternalSaleID:     externalSaleID,
		CustomerMobileE164: customerMobileE164,
		CustomerEmail:      customerEmail,
		PeriodStart:        sub.PeriodStart().UTC(),
		PeriodEnd:          sub.PeriodEnd().UTC(),
		PaidAt:             sub.LastEventAt().UTC(),
		OccurredAt:         sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionActivated, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) PublishActivatedWithoutToken(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
	customerMobileE164 string,
	customerEmail string,
	externalSaleID string,
) error {
	payload := SubscriptionActivatedWithoutTokenPayload{
		SubscriptionID:     subscriptionID,
		PlanCode:           string(sub.Plan().Code()),
		ExternalSaleID:     externalSaleID,
		CustomerMobileE164: customerMobileE164,
		CustomerEmail:      customerEmail,
		PaidAt:             sub.LastEventAt().UTC(),
		OccurredAt:         sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionActivatedWithoutToken, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) PublishRenewed(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
	previousPeriodEnd time.Time,
) error {
	payload := SubscriptionRenewedPayload{
		SubscriptionID:    subscriptionID,
		PlanCode:          string(sub.Plan().Code()),
		PreviousPeriodEnd: previousPeriodEnd.UTC(),
		PeriodEnd:         sub.PeriodEnd().UTC(),
		OccurredAt:        sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionRenewed, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) PublishPastDue(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
) error {
	payload := SubscriptionPastDuePayload{
		SubscriptionID: subscriptionID,
		PeriodEnd:      sub.PeriodEnd().UTC(),
		GraceEnd:       sub.GraceEnd().UTC(),
		OccurredAt:     sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionPastDue, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) PublishCanceled(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
) error {
	payload := SubscriptionCanceledPayload{
		SubscriptionID: subscriptionID,
		PeriodEnd:      sub.PeriodEnd().UTC(),
		OccurredAt:     sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionCanceled, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) PublishRefunded(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
) error {
	payload := SubscriptionRefundedPayload{
		SubscriptionID: subscriptionID,
		OccurredAt:     sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionRefunded, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) PublishExpired(
	ctx context.Context,
	tx database.DBTX,
	sub entities.Subscription,
	subscriptionID string,
	graceEnd time.Time,
) error {
	payload := SubscriptionExpiredAfterGracePayload{
		SubscriptionID: subscriptionID,
		PeriodEnd:      sub.PeriodEnd().UTC(),
		GraceEnd:       graceEnd.UTC(),
		OccurredAt:     sub.LastEventAt().UTC(),
	}
	if err := p.publish(ctx, tx, subscriptionID, EventTypeSubscriptionExpired, payload); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) publish(
	ctx context.Context,
	tx database.DBTX,
	aggregateID string,
	eventType string,
	payload any,
) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:            p.idGen.NewID(),
		Type:          eventType,
		AggregateType: "Subscription",
		AggregateID:   aggregateID,
		Payload:       raw,
		OccurredAt:    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("new event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(tx)
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)

	if err := publisher.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}
