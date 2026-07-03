package memory

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
)

const EventTypeEmbeddingIndex = "platform.memory.embedding.index.v1"

type publishingMessageStore struct {
	next    MessageStore
	pub     MessageIndexPublisher
	model   string
	o11y    observability.Observability
	indexed observability.Counter
	failed  observability.Counter
}

func NewPublishingMessageStore(next MessageStore, pub MessageIndexPublisher, model string, o11y observability.Observability) MessageStore {
	indexed := o11y.Metrics().Counter(
		"platform_memory_embedding_index_published_total",
		"Total de eventos de indexacao de embedding publicados",
		"1",
	)
	failed := o11y.Metrics().Counter(
		"platform_memory_embedding_index_publish_failed_total",
		"Total de falhas ao publicar evento de indexacao de embedding",
		"1",
	)
	return &publishingMessageStore{
		next:    next,
		pub:     pub,
		model:   model,
		o11y:    o11y,
		indexed: indexed,
		failed:  failed,
	}
}

func (s *publishingMessageStore) Append(ctx context.Context, threadPK uuid.UUID, m Message) error {
	if err := s.next.Append(ctx, threadPK, m); err != nil {
		return err
	}

	p := IndexMessagePayload{
		ResourceID: m.ResourceID,
		ThreadID:   threadPK.String(),
		MessageID:  m.ID,
		Content:    m.Content,
		Model:      s.model,
	}

	if pubErr := s.pub.PublishIndex(ctx, p); pubErr != nil {
		s.failed.Add(ctx, 1, observability.String("model", s.model))
		s.o11y.Logger().Error(ctx, "platform.memory.publishing_store.publish_index.failed",
			observability.String("resource_id", m.ResourceID),
			observability.String("model", s.model),
			observability.Error(fmt.Errorf("platform.memory.publishing_store: %w", pubErr)),
		)
		return nil
	}

	s.indexed.Add(ctx, 1, observability.String("model", s.model))
	return nil
}

func (s *publishingMessageStore) Recent(ctx context.Context, threadPK uuid.UUID, limit int) ([]Message, error) {
	return s.next.Recent(ctx, threadPK, limit)
}
