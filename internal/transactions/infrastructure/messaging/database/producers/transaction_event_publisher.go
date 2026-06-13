package producers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

const aggregateTypeTransaction = "transactions.transaction"

const eventTypeTransactionCreated = "transactions.transaction.created.v1"
const eventTypeTransactionUpdated = "transactions.transaction.updated.v1"
const eventTypeTransactionDeleted = "transactions.transaction.deleted.v1"

type transactionEventPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

func NewTransactionEventPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	o11y observability.Observability,
) interfaces.TransactionEventPublisher {
	return &transactionEventPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		o11y:          o11y,
	}
}

func (p *transactionEventPublisher) PublishCreated(ctx context.Context, db database.DBTX, evt entities.TransactionCreated) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal created: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:              evt.EventID.String(),
		Type:            eventTypeTransactionCreated,
		AggregateType:   aggregateTypeTransaction,
		AggregateID:     evt.AggregateID.String(),
		AggregateUserID: evt.UserID.String(),
		Payload:         raw,
		OccurredAt:      evt.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event created: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("transactions/producer: publish created: %w", err)
	}
	return nil
}

func (p *transactionEventPublisher) PublishUpdated(ctx context.Context, db database.DBTX, evt entities.TransactionUpdated) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal updated: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:              evt.EventID.String(),
		Type:            eventTypeTransactionUpdated,
		AggregateType:   aggregateTypeTransaction,
		AggregateID:     evt.AggregateID.String(),
		AggregateUserID: evt.UserID.String(),
		Payload:         raw,
		OccurredAt:      evt.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event updated: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("transactions/producer: publish updated: %w", err)
	}
	return nil
}

func (p *transactionEventPublisher) PublishDeleted(ctx context.Context, db database.DBTX, evt entities.TransactionDeleted) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal deleted: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:              evt.EventID.String(),
		Type:            eventTypeTransactionDeleted,
		AggregateType:   aggregateTypeTransaction,
		AggregateID:     evt.AggregateID.String(),
		AggregateUserID: evt.UserID.String(),
		Payload:         raw,
		OccurredAt:      evt.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event deleted: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("transactions/producer: publish deleted: %w", err)
	}
	return nil
}
