package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type recurringMaterializationRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewRecurringMaterializationRepository(o11y observability.Observability, db database.DBTX) interfaces.RecurringMaterializationRepository {
	return &recurringMaterializationRepository{db: db, o11y: o11y}
}

func (r *recurringMaterializationRepository) TryAdvisoryLock(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth) (bool, func(), error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_materialization.try_advisory_lock")
	defer span.End()

	const q = `SELECT pg_try_advisory_xact_lock(hashtext($1 || $2))`

	rows, err := r.db.QueryContext(ctx, q, templateID.String(), refMonth.String())
	if err != nil {
		span.RecordError(err)
		return false, nil, fmt.Errorf("transactions/repository: advisory lock: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var acquired bool
	if rows.Next() {
		if scanErr := rows.Scan(&acquired); scanErr != nil {
			return false, nil, fmt.Errorf("transactions/repository: scan advisory lock: %w", scanErr)
		}
	}
	if err := rows.Err(); err != nil {
		return false, nil, fmt.Errorf("transactions/repository: rows advisory lock: %w", err)
	}

	return acquired, func() {}, nil
}

func (r *recurringMaterializationRepository) InsertIfAbsent(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth, materializedTransactionID, materializedPurchaseID *uuid.UUID, now time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_materialization.insert_if_absent")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.transactions_recurring_materializations
			(template_id, ref_month, materialized_transaction_id, materialized_purchase_id, materialized_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (template_id, ref_month) DO NOTHING
	`

	res, err := r.db.ExecContext(ctx, q, templateID, refMonth.String(), materializedTransactionID, materializedPurchaseID, now)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("transactions/repository: insert materialização: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("transactions/repository: rows affected materialização: %w", err)
	}

	return rows > 0, nil
}

func (r *recurringMaterializationRepository) IsCompleted(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_materialization.is_completed")
	defer span.End()

	const q = `
		SELECT materialized_transaction_id IS NOT NULL OR materialized_purchase_id IS NOT NULL
		FROM mecontrola.transactions_recurring_materializations
		WHERE template_id = $1 AND ref_month = $2
	`

	rows, err := r.db.QueryContext(ctx, q, templateID, refMonth.String())
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("transactions/repository: is_completed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if rows.Next() {
		var completed bool
		if scanErr := rows.Scan(&completed); scanErr != nil {
			return false, fmt.Errorf("transactions/repository: scan is_completed: %w", scanErr)
		}
		return completed, rows.Err()
	}
	return false, rows.Err()
}

func (r *recurringMaterializationRepository) MarkCompleted(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth, transactionID, purchaseID *uuid.UUID) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_materialization.mark_completed")
	defer span.End()

	const q = `
		UPDATE mecontrola.transactions_recurring_materializations
		SET materialized_transaction_id = $3, materialized_purchase_id = $4
		WHERE template_id = $1 AND ref_month = $2
		  AND materialized_transaction_id IS NULL AND materialized_purchase_id IS NULL
	`

	_, err := r.db.ExecContext(ctx, q, templateID, refMonth.String(), transactionID, purchaseID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: mark_completed: %w", err)
	}
	return nil
}
