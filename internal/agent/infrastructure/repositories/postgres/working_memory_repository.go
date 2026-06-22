package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type workingMemoryRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewWorkingMemoryRepository(o11y observability.Observability, db database.DBTX) interfaces.WorkingMemoryRepository {
	return &workingMemoryRepository{o11y: o11y, db: db}
}

func (r *workingMemoryRepository) Get(ctx context.Context, userID uuid.UUID) (entities.WorkingMemory, bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.working_memory.get")
	defer span.End()

	const query = `
		SELECT content, updated_at
		  FROM mecontrola.agent_working_memory
		 WHERE user_id = $1
	`

	var wm entities.WorkingMemory
	wm.UserID = userID
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&wm.Content, &wm.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.WorkingMemory{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return entities.WorkingMemory{}, false, fmt.Errorf("agent.repository.pg.working_memory.get: %w", err)
	}
	return wm, true, nil
}

func (r *workingMemoryRepository) Upsert(ctx context.Context, wm entities.WorkingMemory) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.working_memory.upsert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_working_memory (user_id, content, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		   SET content    = EXCLUDED.content,
		       updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.ExecContext(ctx, query, wm.UserID, wm.Content, wm.UpdatedAt.UTC())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agent.repository.pg.working_memory.upsert: %w", err)
	}
	return nil
}
