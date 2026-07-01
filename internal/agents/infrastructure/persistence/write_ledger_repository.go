package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type writeLedgerRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewWriteLedgerRepository(db database.DBTX, o11y observability.Observability) usecases.WriteLedgerRepository {
	return &writeLedgerRepository{db: db, o11y: o11y}
}

func (r *writeLedgerRepository) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *writeLedgerRepository) FindByKey(ctx context.Context, wamid string, itemSeq int, operation string) (usecases.WriteLedgerEntry, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.write_ledger.find_by_key")
	defer span.End()

	const q = `
		SELECT id, user_id, wamid, item_seq, operation, resource_id, resource_kind, created_at
		  FROM mecontrola.agents_write_ledger
		 WHERE wamid = $1 AND item_seq = $2 AND operation = $3
		 LIMIT 1`

	var e usecases.WriteLedgerEntry
	err := r.conn(ctx).QueryRowContext(ctx, q, wamid, itemSeq, operation).Scan(
		&e.ID, &e.UserID, &e.WAMID, &e.ItemSeq, &e.Operation,
		&e.ResourceID, &e.ResourceKind, &e.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return usecases.WriteLedgerEntry{}, usecases.ErrLedgerEntryNotFound
	}
	if err != nil {
		span.RecordError(err)
		return usecases.WriteLedgerEntry{}, fmt.Errorf("agents.persistence.write_ledger.find_by_key: %w", err)
	}
	return e, nil
}

func (r *writeLedgerRepository) Insert(ctx context.Context, entry usecases.WriteLedgerEntry) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.write_ledger.insert")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.agents_write_ledger
			(id, user_id, wamid, item_seq, operation, resource_id, resource_kind, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (wamid, item_seq, operation) DO NOTHING`

	_, err := r.conn(ctx).ExecContext(ctx, q,
		entry.ID,
		entry.UserID,
		entry.WAMID,
		entry.ItemSeq,
		entry.Operation,
		entry.ResourceID,
		entry.ResourceKind,
		entry.CreatedAt,
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil
		}
		return fmt.Errorf("agents.persistence.write_ledger.insert: %w", err)
	}
	return nil
}

func (r *writeLedgerRepository) DeleteBefore(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.persistence.write_ledger.delete_before")
	defer span.End()

	const q = `
		DELETE FROM mecontrola.agents_write_ledger
		 WHERE id IN (
			SELECT id FROM mecontrola.agents_write_ledger
			 WHERE created_at <= $1
			 ORDER BY created_at
			 LIMIT $2
		)`

	result, err := r.conn(ctx).ExecContext(ctx, q, before, batchSize)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("agents.persistence.write_ledger.delete_before: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("agents.persistence.write_ledger.delete_before.rows_affected: %w", err)
	}
	return rows, nil
}
