package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SubscriptionEventPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
	o11y          observability.Observability
}

func NewSubscriptionEventPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
	o11y observability.Observability,
) *SubscriptionEventPublisher {
	return &SubscriptionEventPublisher{outboxFactory: outboxFactory, cfg: cfg, idGen: idGen, o11y: o11y}
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionActivated, payload, sub.LastEventAt()); err != nil {
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionActivatedWithoutToken, payload, sub.LastEventAt()); err != nil {
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionRenewed, payload, sub.LastEventAt()); err != nil {
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionPastDue, payload, sub.LastEventAt()); err != nil {
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionCanceled, payload, sub.LastEventAt()); err != nil {
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionRefunded, payload, sub.LastEventAt()); err != nil {
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
	if err := p.publish(ctx, tx, subscriptionID, sub.UserID(), EventTypeSubscriptionExpired, payload, sub.LastEventAt()); err != nil {
		return fmt.Errorf("billing/producer: %w", err)
	}
	return nil
}

func (p *SubscriptionEventPublisher) publish(
	ctx context.Context,
	tx database.DBTX,
	aggregateID string,
	aggregateUserID string,
	eventType string,
	payload any,
	occurredAt time.Time,
) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:              p.idGen.NewID(),
		Type:            eventType,
		AggregateType:   "Subscription",
		AggregateID:     aggregateID,
		AggregateUserID: aggregateUserID,
		Payload:         raw,
		OccurredAt:      occurredAt,
	})
	if err != nil {
		return fmt.Errorf("new event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(tx)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)

	if err := publisher.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}
