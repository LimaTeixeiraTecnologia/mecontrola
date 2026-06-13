package outbox

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type postgresPublisher struct {
	storage       Storage
	cfg           configs.OutboxConfig
	insertedTotal observability.Counter
	o11y          observability.Observability
}

func NewPostgresPublisher(storage Storage, cfg configs.OutboxConfig) Publisher {
	return &postgresPublisher{storage: storage, cfg: cfg}
}

func NewObservablePostgresPublisher(storage Storage, cfg configs.OutboxConfig, o11y observability.Observability) Publisher {
	insertedTotal := o11y.Metrics().Counter(
		"outbox_events_inserted_total",
		"Total de eventos inseridos no outbox por presenca de aggregate_user_id",
		"1",
	)
	return &postgresPublisher{
		storage:       storage,
		cfg:           cfg,
		o11y:          o11y,
		insertedTotal: insertedTotal,
	}
}

func (p *postgresPublisher) Publish(ctx context.Context, evt Event) error {
	if _, err := uuid.Parse(evt.ID); err != nil {
		return ErrEventIDMissing
	}
	if evt.Type == "" {
		return ErrEventTypeMissing
	}
	if evt.AggregateType == "" {
		return ErrAggregateTypeMissing
	}
	if evt.AggregateID == "" {
		return ErrAggregateIDMissing
	}
	if !json.Valid(evt.Payload) {
		return ErrInvalidPayload
	}
	trimmed := bytes.TrimLeft(evt.Payload, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ErrInvalidPayload
	}
	if evt.OccurredAt.IsZero() {
		return ErrOccurredAtZero
	}

	if err := p.storage.Insert(ctx, evt, p.cfg.RetryMaxAttempts); err != nil {
		return err
	}

	if p.insertedTotal != nil {
		hasUserID := "false"
		if evt.AggregateUserID != "" {
			hasUserID = "true"
		}
		p.insertedTotal.Add(ctx, 1, observability.String("has_user_id", hasUserID))
	}
	return nil
}
