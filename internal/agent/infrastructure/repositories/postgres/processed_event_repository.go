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

const prefixProcessedEventRepository = "agent.processed_event.repository.pg:"

type processedEventRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewProcessedEventRepository(o11y observability.Observability, db database.DBTX) interfaces.ProcessedEventRepository {
	return &processedEventRepository{o11y: o11y, db: db}
}

func (r *processedEventRepository) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.processed_event.repository.pg.is_processed")
	defer span.End()

	const query = `
		SELECT EXISTS(
			SELECT 1
			  FROM mecontrola.agent_processed_events
			 WHERE event_id = $1
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("%s is_processed: %w", prefixProcessedEventRepository, err)
	}
	return true, nil
}

func (r *processedEventRepository) MarkProcessed(ctx context.Context, eventID uuid.UUID, eventType string, userID uuid.UUID, processedAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.processed_event.repository.pg.mark_processed")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_processed_events (event_id, event_type, aggregate_user_id, processed_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query, eventID, eventType, nullableUUID(userID, userID != uuid.Nil), processedAt.UTC())
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%s %w", prefixProcessedEventRepository, interfaces.ErrProcessedEventAlreadyExists)
		}
		return fmt.Errorf("%s mark_processed: %w", prefixProcessedEventRepository, err)
	}
	return nil
}
