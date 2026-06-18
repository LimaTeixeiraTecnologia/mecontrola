package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type processedEventRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewProcessedEventRepository(o11y observability.Observability, db database.DBTX) interfaces.ProcessedEventRepository {
	return &processedEventRepository{o11y: o11y, db: db}
}

func (r *processedEventRepository) MarkApplied(ctx context.Context, eventKey string, trigger string, recursoID string, occurredAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.processed_event.mark_applied")
	defer span.End()

	const query = `
		INSERT INTO billing_processed_events (event_key, trigger, recurso_id, occurred_at, status)
		VALUES ($1, $2, $3, $4, 'applied')
	`

	_, err := r.db.ExecContext(ctx, query, eventKey, trigger, recursoID, occurredAt)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("billing/postgres: mark_applied: %w", interfaces.ErrEventAlreadyProcessed)
		}
		return fmt.Errorf("billing/postgres: mark_applied: %w", err)
	}
	return nil
}

func (r *processedEventRepository) MarkSuperseded(ctx context.Context, eventKey string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.processed_event.mark_superseded")
	defer span.End()

	const query = `
		UPDATE billing_processed_events
		   SET status = 'superseded'
		 WHERE event_key = $1
	`

	_, err := r.db.ExecContext(ctx, query, eventKey)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("billing/postgres: mark_superseded: %w", err)
	}
	return nil
}
