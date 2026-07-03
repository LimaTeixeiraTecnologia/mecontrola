package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type EmbeddingIndexHandler struct {
	embedder  Embedder
	recall    memory.SemanticRecall
	o11y      observability.Observability
	succeeded observability.Counter
	failed    observability.Counter
}

func NewEmbeddingIndexHandler(embedder Embedder, recall memory.SemanticRecall, o11y observability.Observability) *EmbeddingIndexHandler {
	succeeded := o11y.Metrics().Counter(
		"platform_memory_embedding_index_succeeded_total",
		"Total de embeddings indexados com sucesso",
		"1",
	)
	failed := o11y.Metrics().Counter(
		"platform_memory_embedding_index_failed_total",
		"Total de falhas na indexacao de embeddings",
		"1",
	)
	return &EmbeddingIndexHandler{
		embedder:  embedder,
		recall:    recall,
		o11y:      o11y,
		succeeded: succeeded,
		failed:    failed,
	}
}

func (h *EmbeddingIndexHandler) Handle(ctx context.Context, event events.Event) error {
	p, err := h.parsePayload(event)
	if err != nil {
		h.failed.Add(ctx, 1, observability.String("reason", "parse"))
		return fmt.Errorf("indexer.handler: %w", err)
	}

	if p.Content == "" {
		return nil
	}

	embeddings, err := h.embedder.Embed(ctx, []string{p.Content})
	if err != nil {
		h.failed.Add(ctx, 1, observability.String("reason", "embed"))
		h.o11y.Logger().Error(ctx, "platform.memory.indexer.handler.embed.failed",
			observability.String("resource_id", p.ResourceID),
			observability.Error(err),
		)
		return fmt.Errorf("indexer.handler: embed: %w", err)
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		h.failed.Add(ctx, 1, observability.String("reason", "empty_embedding"))
		return fmt.Errorf("indexer.handler: empty embedding returned")
	}

	if err := h.recall.Index(ctx, p.ResourceID, p.ThreadID, p.MessageID, p.Content, p.Model, embeddings[0]); err != nil {
		h.failed.Add(ctx, 1, observability.String("reason", "index"))
		h.o11y.Logger().Error(ctx, "platform.memory.indexer.handler.index.failed",
			observability.String("resource_id", p.ResourceID),
			observability.Error(err),
		)
		return fmt.Errorf("indexer.handler: index: %w", err)
	}

	h.succeeded.Add(ctx, 1, observability.String("model", p.Model))
	return nil
}

func (h *EmbeddingIndexHandler) parsePayload(event events.Event) (memory.IndexMessagePayload, error) {
	payload := event.GetPayload()

	env, ok := payload.(outbox.Envelope)
	if ok {
		var p memory.IndexMessagePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return memory.IndexMessagePayload{}, fmt.Errorf("unmarshal envelope payload: %w", err)
		}
		return p, nil
	}

	var raw []byte
	switch v := payload.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		b, err := json.Marshal(payload)
		if err != nil {
			return memory.IndexMessagePayload{}, fmt.Errorf("marshal payload: %w", err)
		}
		raw = b
	}

	var p memory.IndexMessagePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return memory.IndexMessagePayload{}, fmt.Errorf("unmarshal payload: %w", err)
	}
	return p, nil
}
