package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type threadRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewThreadRepository(db database.DBTX, o11y observability.Observability) memory.ThreadGateway {
	return &threadRepository{db: db, o11y: o11y}
}

func (r *threadRepository) GetOrCreate(ctx context.Context, resourceID, threadID string) (memory.Thread, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.thread.get_or_create")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id, title, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, '', '{}'::jsonb, now(), now())
		ON CONFLICT (resource_id, thread_id) DO UPDATE
			SET updated_at = mecontrola.platform_threads.updated_at
		RETURNING id, resource_id, thread_id, title, metadata, created_at, updated_at`

	newID := uuid.New()
	var (
		id        uuid.UUID
		resID     string
		thrID     string
		title     string
		metaBytes []byte
		createdAt time.Time
		updatedAt time.Time
	)

	err := r.db.QueryRowContext(ctx, q, newID, resourceID, threadID).
		Scan(&id, &resID, &thrID, &title, &metaBytes, &createdAt, &updatedAt)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "platform.memory.repository.thread.get_or_create.failed",
			observability.String("resource_id", resourceID),
			observability.String("thread_id", threadID),
			observability.Error(err),
		)
		return memory.Thread{}, fmt.Errorf("platform.memory.repository.thread.get_or_create: %w", err)
	}

	meta, err := unmarshalMetadata(metaBytes)
	if err != nil {
		return memory.Thread{}, fmt.Errorf("platform.memory.repository.thread.get_or_create: %w", err)
	}

	return memory.Thread{
		ID:         id,
		ResourceID: resID,
		ThreadID:   thrID,
		Title:      title,
		Metadata:   meta,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func unmarshalMetadata(b []byte) (map[string]any, error) {
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	m := make(map[string]any)
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return m, nil
}
