package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const prefixAgentSessionRepository = "agent.repository.pg:"

type agentSessionRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewAgentSessionRepository(o11y observability.Observability, db database.DBTX) interfaces.AgentSessionRepository {
	return &agentSessionRepository{o11y: o11y, db: db}
}

func (r *agentSessionRepository) Create(ctx context.Context, record interfaces.AgentSessionRecord) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.create")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_sessions
		       (id, user_id, channel, pending_action, recent_turns, created_at, updated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		record.ID,
		record.UserID,
		record.Channel,
		jsonOrDefault(record.PendingAction, "{}"),
		jsonOrDefault(record.RecentTurns, "[]"),
		record.CreatedAt.UTC(),
		record.UpdatedAt.UTC(),
		record.ExpiresAt.UTC(),
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%s %w", prefixAgentSessionRepository, interfaces.ErrAgentSessionConflict)
		}
		return fmt.Errorf("%s create: %w", prefixAgentSessionRepository, err)
	}
	return nil
}

func (r *agentSessionRepository) GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (interfaces.AgentSessionRecord, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.get_by_user_and_channel")
	defer span.End()

	const query = `
		SELECT id, user_id, channel, pending_action, recent_turns, created_at, updated_at, expires_at
		  FROM mecontrola.agent_sessions
		 WHERE user_id = $1 AND channel = $2 AND expires_at > now()
	`

	var (
		record        interfaces.AgentSessionRecord
		pendingAction []byte
		recentTurns   []byte
	)

	err := r.db.QueryRowContext(ctx, query, userID, channel).Scan(
		&record.ID,
		&record.UserID,
		&record.Channel,
		&pendingAction,
		&recentTurns,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return interfaces.AgentSessionRecord{}, fmt.Errorf("%s %w", prefixAgentSessionRepository, interfaces.ErrAgentSessionNotFound)
	}
	if err != nil {
		span.RecordError(err)
		return interfaces.AgentSessionRecord{}, fmt.Errorf("%s get_by_user_and_channel: %w", prefixAgentSessionRepository, err)
	}

	record.PendingAction = pendingAction
	record.RecentTurns = recentTurns
	return record, nil
}

func (r *agentSessionRepository) Update(ctx context.Context, record interfaces.AgentSessionRecord) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.update")
	defer span.End()

	const query = `
		UPDATE mecontrola.agent_sessions
		   SET pending_action = $1,
		       recent_turns   = $2,
		       updated_at     = $3,
		       expires_at     = $4
		 WHERE id = $5
	`

	result, err := r.db.ExecContext(ctx, query,
		jsonOrDefault(record.PendingAction, "{}"),
		jsonOrDefault(record.RecentTurns, "[]"),
		record.UpdatedAt.UTC(),
		record.ExpiresAt.UTC(),
		record.ID,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s update: %w", prefixAgentSessionRepository, err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		span.RecordError(rowsErr)
		return fmt.Errorf("%s update rows_affected: %w", prefixAgentSessionRepository, rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("%s %w", prefixAgentSessionRepository, interfaces.ErrAgentSessionNotFound)
	}
	return nil
}

func (r *agentSessionRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.pg.delete_expired")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.agent_sessions
		 WHERE expires_at <= $1
	`

	result, err := r.db.ExecContext(ctx, query, before.UTC())
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("%s delete_expired: %w", prefixAgentSessionRepository, err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		span.RecordError(rowsErr)
		return 0, fmt.Errorf("%s delete_expired rows_affected: %w", prefixAgentSessionRepository, rowsErr)
	}
	return affected, nil
}

func jsonOrDefault(raw []byte, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}
