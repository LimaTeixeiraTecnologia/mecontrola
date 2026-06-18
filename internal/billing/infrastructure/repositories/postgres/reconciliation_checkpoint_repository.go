package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type reconciliationCheckpointRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewReconciliationCheckpointRepository(o11y observability.Observability, db database.DBTX) interfaces.ReconciliationCheckpointRepository {
	return &reconciliationCheckpointRepository{o11y: o11y, db: db}
}

func (r *reconciliationCheckpointRepository) Get(ctx context.Context, name string) (time.Time, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.reconciliation_checkpoint.get")
	defer span.End()

	const query = `
		SELECT watermark
		  FROM billing_reconciliation_checkpoints
		 WHERE name = $1
		   FOR UPDATE
	`

	var watermark time.Time
	err := r.db.QueryRowContext(ctx, query, name).Scan(&watermark)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf("billing/postgres: get_checkpoint: %w", application.ErrCheckpointNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "billing.repository.reconciliation_checkpoint.get_failed",
			observability.String("operation", "get"),
			observability.String("name", name),
			observability.Error(err),
		)
		return time.Time{}, fmt.Errorf("billing/postgres: get_checkpoint: %w", err)
	}
	return watermark, nil
}

func (r *reconciliationCheckpointRepository) Set(ctx context.Context, name string, watermark time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.reconciliation_checkpoint.set")
	defer span.End()

	const query = `
		INSERT INTO billing_reconciliation_checkpoints (name, watermark, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (name) DO UPDATE SET
		    watermark  = EXCLUDED.watermark,
		    updated_at = now()
	`

	_, err := r.db.ExecContext(ctx, query, name, watermark)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "billing.repository.reconciliation_checkpoint.set_failed",
			observability.String("operation", "set"),
			observability.String("name", name),
			observability.Error(err),
		)
		return fmt.Errorf("billing/postgres: set_checkpoint: %w", err)
	}
	return nil
}
