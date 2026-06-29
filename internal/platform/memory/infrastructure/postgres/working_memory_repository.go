package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type workingMemoryRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewWorkingMemoryRepository(db database.DBTX, o11y observability.Observability) memory.WorkingMemory {
	return &workingMemoryRepository{db: db, o11y: o11y}
}

func (r *workingMemoryRepository) Get(ctx context.Context, resourceID string) (string, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.working_memory.get")
	defer span.End()

	const q = `
		SELECT working_memory
		  FROM mecontrola.platform_resources
		 WHERE resource_id = $1`

	var content string
	err := r.db.QueryRowContext(ctx, q, resourceID).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("platform.memory.repository.working_memory.get: %w", memory.ErrWorkingMemoryNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "platform.memory.repository.working_memory.get.failed",
			observability.String("resource_id", resourceID),
			observability.Error(err),
		)
		return "", fmt.Errorf("platform.memory.repository.working_memory.get: %w", err)
	}

	return content, nil
}

func (r *workingMemoryRepository) Upsert(ctx context.Context, resourceID, content string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "platform.memory.repository.working_memory.upsert")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.platform_resources (resource_id, working_memory, metadata, updated_at)
		VALUES ($1, $2, '{}'::jsonb, now())
		ON CONFLICT (resource_id) DO UPDATE
			SET working_memory = EXCLUDED.working_memory,
			    updated_at     = now()`

	_, err := r.db.ExecContext(ctx, q, resourceID, content)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "platform.memory.repository.working_memory.upsert.failed",
			observability.String("resource_id", resourceID),
			observability.Error(err),
		)
		return fmt.Errorf("platform.memory.repository.working_memory.upsert: %w", err)
	}

	return nil
}
