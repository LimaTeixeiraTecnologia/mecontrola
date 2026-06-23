package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const prefixAgentThreadRepository = "agent.thread.repository.pg:"

type agentThreadRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewAgentThreadRepository(o11y observability.Observability, db database.DBTX) interfaces.AgentThreadRepository {
	return &agentThreadRepository{o11y: o11y, db: db}
}

func (r *agentThreadRepository) GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (entities.Thread, bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.thread.get_by_user_and_channel")
	defer span.End()

	const query = `
		SELECT id, user_id, channel, created_at, updated_at
		  FROM mecontrola.agent_threads
		 WHERE user_id = $1 AND channel = $2
	`

	var (
		id        uuid.UUID
		uid       uuid.UUID
		ch        string
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx, query, userID, channel).Scan(&id, &uid, &ch, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Thread{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return entities.Thread{}, false, fmt.Errorf("%s get_by_user_and_channel: %w", prefixAgentThreadRepository, err)
	}

	thread, restoreErr := entities.RestoreThread(entities.ThreadParams{
		ID:        id,
		UserID:    uid,
		Channel:   ch,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})
	if restoreErr != nil {
		span.RecordError(restoreErr)
		return entities.Thread{}, false, fmt.Errorf("%s restore: %w", prefixAgentThreadRepository, restoreErr)
	}
	return thread, true, nil
}

func (r *agentThreadRepository) Upsert(ctx context.Context, thread entities.Thread) (entities.Thread, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.thread.upsert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_threads (id, user_id, channel, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, channel)
		DO UPDATE SET updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, channel, created_at, updated_at
	`

	var (
		id        uuid.UUID
		uid       uuid.UUID
		ch        string
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx, query,
		thread.ID(),
		thread.UserID(),
		thread.Channel(),
		thread.CreatedAt().UTC(),
		thread.UpdatedAt().UTC(),
	).Scan(&id, &uid, &ch, &createdAt, &updatedAt)
	if err != nil {
		span.RecordError(err)
		return entities.Thread{}, fmt.Errorf("%s upsert: %w", prefixAgentThreadRepository, err)
	}

	persisted, restoreErr := entities.RestoreThread(entities.ThreadParams{
		ID:        id,
		UserID:    uid,
		Channel:   ch,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})
	if restoreErr != nil {
		span.RecordError(restoreErr)
		return entities.Thread{}, fmt.Errorf("%s restore: %w", prefixAgentThreadRepository, restoreErr)
	}
	return persisted, nil
}
