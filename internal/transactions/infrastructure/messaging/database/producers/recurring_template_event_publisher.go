package producers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

const aggregateTypeRecurringTemplate = "transactions.recurring_template"

const eventTypeRecurringTemplateCreated = "transactions.recurring_template.created.v1"
const eventTypeRecurringTemplateUpdated = "transactions.recurring_template.updated.v1"
const eventTypeRecurringTemplateDeleted = "transactions.recurring_template.deleted.v1"

type recurringTemplateEventPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
}

func NewRecurringTemplateEventPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
) interfaces.RecurringTemplateEventPublisher {
	return &recurringTemplateEventPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
	}
}

func (p *recurringTemplateEventPublisher) PublishCreated(ctx context.Context, db database.DBTX, evt entities.RecurringTemplateCreated) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal recurring_template created: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:            evt.EventID.String(),
		Type:          eventTypeRecurringTemplateCreated,
		AggregateType: aggregateTypeRecurringTemplate,
		AggregateID:   evt.AggregateID.String(),
		Payload:       raw,
		OccurredAt:    evt.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event recurring_template created: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("transactions/producer: publish recurring_template created: %w", err)
	}
	return nil
}

func (p *recurringTemplateEventPublisher) PublishUpdated(ctx context.Context, db database.DBTX, evt entities.RecurringTemplateUpdated) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal recurring_template updated: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:            evt.EventID.String(),
		Type:          eventTypeRecurringTemplateUpdated,
		AggregateType: aggregateTypeRecurringTemplate,
		AggregateID:   evt.AggregateID.String(),
		Payload:       raw,
		OccurredAt:    evt.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event recurring_template updated: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("transactions/producer: publish recurring_template updated: %w", err)
	}
	return nil
}

func (p *recurringTemplateEventPublisher) PublishDeleted(ctx context.Context, db database.DBTX, evt entities.RecurringTemplateDeleted) error {
	raw, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("transactions/producer: marshal recurring_template deleted: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:            evt.EventID.String(),
		Type:          eventTypeRecurringTemplateDeleted,
		AggregateType: aggregateTypeRecurringTemplate,
		AggregateID:   evt.AggregateID.String(),
		Payload:       raw,
		OccurredAt:    evt.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("transactions/producer: new event recurring_template deleted: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("transactions/producer: publish recurring_template deleted: %w", err)
	}
	return nil
}
