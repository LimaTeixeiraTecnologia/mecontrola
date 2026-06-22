package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type observationRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewObservationRepository(o11y observability.Observability, db database.DBTX) interfaces.ObservationRepository {
	return &observationRepository{o11y: o11y, db: db}
}

func (r *observationRepository) Insert(ctx context.Context, obs entities.Observation) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.observation.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_observations (id, user_id, channel, content, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(ctx, query,
		obs.ID,
		obs.UserID,
		obs.Channel,
		obs.Content,
		obs.CreatedAt.UTC(),
		obs.ExpiresAt.UTC(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agent.repository.pg.observation.insert: %w", err)
	}
	return nil
}

func (r *observationRepository) ListRecent(ctx context.Context, userID uuid.UUID, channel string, limit int) ([]entities.Observation, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.observation.list_recent")
	defer span.End()

	const query = `
		SELECT id, user_id, channel, content, created_at, expires_at
		  FROM mecontrola.agent_observations
		 WHERE user_id = $1 AND channel = $2 AND expires_at > now()
		 ORDER BY created_at ASC
		 LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, channel, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agent.repository.pg.observation.list_recent: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var obs []entities.Observation
	for rows.Next() {
		var o entities.Observation
		if scanErr := rows.Scan(&o.ID, &o.UserID, &o.Channel, &o.Content, &o.CreatedAt, &o.ExpiresAt); scanErr != nil {
			return nil, fmt.Errorf("agent.repository.pg.observation.list_recent scan: %w", scanErr)
		}
		obs = append(obs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("agent.repository.pg.observation.list_recent rows: %w", err)
	}
	return obs, nil
}

func (r *observationRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.observation.delete_expired")
	defer span.End()

	const query = `DELETE FROM mecontrola.agent_observations WHERE expires_at <= $1`

	result, err := r.db.ExecContext(ctx, query, before.UTC())
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("agent.repository.pg.observation.delete_expired: %w", err)
	}
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return 0, fmt.Errorf("agent.repository.pg.observation.delete_expired rows_affected: %w", rowsErr)
	}
	return affected, nil
}

func (r *observationRepository) DeleteOldestBeyondLimit(ctx context.Context, userID uuid.UUID, channel string, keep int) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.observation.delete_oldest_beyond_limit")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.agent_observations
		 WHERE user_id = $1 AND channel = $2
		   AND id NOT IN (
		       SELECT id FROM mecontrola.agent_observations
		        WHERE user_id = $1 AND channel = $2
		          AND expires_at > now()
		        ORDER BY created_at DESC
		        LIMIT $3
		   )
	`

	_, err := r.db.ExecContext(ctx, query, userID, channel, keep)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agent.repository.pg.observation.delete_oldest_beyond_limit: %w", err)
	}
	return nil
}
