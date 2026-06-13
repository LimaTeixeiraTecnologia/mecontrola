package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

const (
	aggregateTypeCardPurchase    = "transactions.card_purchase"
	eventTypeCardPurchaseCreated = "transactions.card_purchase.created.v1"
	eventTypeCardPurchaseUpdated = "transactions.card_purchase.updated.v1"
	eventTypeCardPurchaseDeleted = "transactions.card_purchase.deleted.v1"
)

type CardPurchaseEventPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

func NewCardPurchaseEventPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	o11y observability.Observability,
) *CardPurchaseEventPublisher {
	return &CardPurchaseEventPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		o11y:          o11y,
	}
}

func (p *CardPurchaseEventPublisher) PublishCreated(ctx context.Context, db database.DBTX, evt entities.CardPurchaseCreated) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal card_purchase.created: %w", err)
	}
	return p.publish(ctx, db, evt.EventID.String(), eventTypeCardPurchaseCreated, evt.AggregateID.String(), evt.UserID.String(), raw, evt.OccurredAt)
}

func (p *CardPurchaseEventPublisher) PublishUpdated(ctx context.Context, db database.DBTX, evt entities.CardPurchaseUpdated) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal card_purchase.updated: %w", err)
	}
	return p.publish(ctx, db, evt.EventID.String(), eventTypeCardPurchaseUpdated, evt.AggregateID.String(), evt.UserID.String(), raw, evt.OccurredAt)
}

func (p *CardPurchaseEventPublisher) PublishDeleted(ctx context.Context, db database.DBTX, evt entities.CardPurchaseDeleted) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal card_purchase.deleted: %w", err)
	}
	return p.publish(ctx, db, evt.EventID.String(), eventTypeCardPurchaseDeleted, evt.AggregateID.String(), evt.UserID.String(), raw, evt.OccurredAt)
}

func (p *CardPurchaseEventPublisher) publish(
	ctx context.Context,
	db database.DBTX,
	eventID, eventType, aggregateID, userID string,
	payload []byte,
	occurredAt time.Time,
) error {
	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:              eventID,
		Type:            eventType,
		AggregateType:   aggregateTypeCardPurchase,
		AggregateID:     aggregateID,
		AggregateUserID: userID,
		Payload:         payload,
		OccurredAt:      occurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if pubErr := publisher.Publish(ctx, evt); pubErr != nil {
		return fmt.Errorf("transactions/producer: publish: %w", pubErr)
	}
	return nil
}
