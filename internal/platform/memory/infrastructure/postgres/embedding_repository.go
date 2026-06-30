package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type embeddingRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewEmbeddingRepository(db database.DBTX, o11y observability.Observability) memory.SemanticRecall {
	return &embeddingRepository{db: db, o11y: o11y}
}

func (r *embeddingRepository) Index(ctx context.Context, resourceID, threadID string, sourceMessagePK uuid.UUID, content, model string, embedding []float32) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.embedding.index")
	defer span.End()

	vec := formatVector(embedding)

	const q = `
		INSERT INTO mecontrola.platform_embeddings (id, resource_id, thread_id, source_message_pk, content, embedding, model, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::vector, $7, now())
		ON CONFLICT (source_message_pk, model) WHERE source_message_pk IS NOT NULL DO NOTHING`

	var srcMsgPK any
	if sourceMessagePK != uuid.Nil {
		srcMsgPK = sourceMessagePK
	}

	_, err := r.db.ExecContext(ctx, q,
		uuid.New(),
		resourceID,
		threadID,
		srcMsgPK,
		content,
		vec,
		model,
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "platform.memory.repository.embedding.index.failed",
			observability.String("resource_id", resourceID),
			observability.Error(err),
		)
		return fmt.Errorf("platform.memory.repository.embedding.index: %w", err)
	}

	return nil
}

func (r *embeddingRepository) Recall(ctx context.Context, resourceID, query string, embedding []float32, k int) ([]memory.RecallHit, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.embedding.recall")
	defer span.End()

	vec := formatVector(embedding)

	const q = `
		SELECT resource_id, thread_id, content,
		       1 - (embedding <=> $1::vector) AS score
		  FROM mecontrola.platform_embeddings
		 WHERE resource_id = $2
		 ORDER BY embedding <=> $1::vector
		 LIMIT $3`

	rows, err := r.db.QueryContext(ctx, q, vec, resourceID, k)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("platform.memory.repository.embedding.recall: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hits []memory.RecallHit
	for rows.Next() {
		var h memory.RecallHit
		if err := rows.Scan(&h.ResourceID, &h.ThreadID, &h.Content, &h.Score); err != nil {
			return nil, fmt.Errorf("platform.memory.repository.embedding.recall.scan: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform.memory.repository.embedding.recall.rows: %w", err)
	}

	return hits, nil
}

func formatVector(v []float32) string {
	sb := strings.Builder{}
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(']')
	return sb.String()
}
