package outbox

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type postgresPublisher struct {
	storage Storage
	cfg     configs.OutboxConfig
}

func NewPostgresPublisher(storage Storage, cfg configs.OutboxConfig) Publisher {
	return &postgresPublisher{storage: storage, cfg: cfg}
}

func (p *postgresPublisher) Publish(ctx context.Context, evt Event) error {
	if _, ok := database.FromContext(ctx); !ok {
		return ErrNoActiveTransaction
	}
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
	return nil
}
