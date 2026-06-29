package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	aggregateTypeMessage = "platform.message"
)

type outboxMessageIndexPublisher struct {
	publisher outbox.Publisher
}

func NewOutboxMessageIndexPublisher(publisher outbox.Publisher) memory.MessageIndexPublisher {
	return &outboxMessageIndexPublisher{publisher: publisher}
}

func (p *outboxMessageIndexPublisher) PublishIndex(ctx context.Context, payload memory.IndexMessagePayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("indexer.outbox_publisher: marshal payload: %w", err)
	}

	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:            uuid.NewString(),
		Type:          memory.EventTypeEmbeddingIndex,
		AggregateType: aggregateTypeMessage,
		AggregateID:   payload.MessagePK.String(),
		Payload:       raw,
		OccurredAt:    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("indexer.outbox_publisher: new event: %w", err)
	}

	if err := p.publisher.Publish(ctx, evt); err != nil {
		return fmt.Errorf("indexer.outbox_publisher: publish: %w", err)
	}

	return nil
}
