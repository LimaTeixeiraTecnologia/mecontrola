package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type expenseRepository struct {
	o11y observability.Observability
}

func NewExpenseRepository(o11y observability.Observability) interfaces.ExpenseRepository {
	return &expenseRepository{o11y: o11y}
}

func (r *expenseRepository) GetByIdentity(ctx context.Context, db database.DBTX, k entities.ExpenseIdentity) (entities.Expense, entities.ExpenseTombstone, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.expense.get_by_identity")
	defer span.End()

	const query = `
		SELECT id, user_id, source, external_transaction_id, subcategory_id,
		       root_slug, competence, amount_cents, occurred_at,
		       version, tombstone_version, deleted_at, created_at, updated_at
		  FROM mecontrola.budgets_expenses
		 WHERE user_id = $1 AND source = $2 AND external_transaction_id = $3
	`

	row := db.QueryRowContext(ctx, query,
		k.UserID, k.Source.String(), k.ExternalTransactionID.String(),
	)

	return r.scanExpenseRow(row)
}

func (r *expenseRepository) Insert(ctx context.Context, db database.DBTX, e entities.Expense) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.expense.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.budgets_expenses
		       (id, user_id, source, external_transaction_id, subcategory_id,
		        root_slug, competence, amount_cents, occurred_at,
		        version, tombstone_version, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := db.ExecContext(ctx, query,
		e.ID(), e.UserID(), e.Source().String(), e.ExternalTransactionID().String(),
		e.SubcategoryID(), e.RootSlug().String(), e.Competence().String(),
		e.AmountCents(), e.OccurredAt(),
		e.Version(), e.TombstoneVersion(), e.DeletedAt(),
		e.CreatedAt(), e.UpdatedAt(),
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("budgets/postgres: insert expense: %w", interfaces.ErrExpenseConflict)
		}
		return fmt.Errorf("budgets/postgres: insert expense: %w", err)
	}
	return nil
}

func (r *expenseRepository) Update(ctx context.Context, db database.DBTX, e entities.Expense, expectedVersion int64) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.expense.update")
	defer span.End()

	const query = `
		UPDATE mecontrola.budgets_expenses
		   SET subcategory_id = $1,
		       root_slug      = $2,
		       competence     = $3,
		       amount_cents   = $4,
		       occurred_at    = $5,
		       version        = $6,
		       updated_at     = $7
		 WHERE id = $8 AND version = $9 AND deleted_at IS NULL
	`

	result, err := db.ExecContext(ctx, query,
		e.SubcategoryID(), e.RootSlug().String(), e.Competence().String(),
		e.AmountCents(), e.OccurredAt(),
		e.Version(), e.UpdatedAt(),
		e.ID(), expectedVersion,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: update expense: %w", err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("budgets/postgres: update expense rows_affected: %w", rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("budgets/postgres: update expense: %w", interfaces.ErrExpenseConflict)
	}
	return nil
}

func (r *expenseRepository) SoftDelete(ctx context.Context, db database.DBTX, e entities.Expense, expectedVersion int64) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.expense.soft_delete")
	defer span.End()

	const query = `
		UPDATE mecontrola.budgets_expenses
		   SET version           = $1,
		       tombstone_version = $2,
		       deleted_at        = $3,
		       updated_at        = $4
		 WHERE id = $5 AND version = $6 AND deleted_at IS NULL
		 RETURNING version
	`

	row := db.QueryRowContext(ctx, query,
		e.Version(), e.TombstoneVersion(), e.DeletedAt(), e.UpdatedAt(),
		e.ID(), expectedVersion,
	)

	var tombstoneVersion int64
	if err := row.Scan(&tombstoneVersion); err != nil {
		span.RecordError(err)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("budgets/postgres: soft_delete expense: %w", interfaces.ErrExpenseConflict)
		}
		return 0, fmt.Errorf("budgets/postgres: soft_delete expense: %w", err)
	}
	return tombstoneVersion, nil
}

func (r *expenseRepository) SumByRoot(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) (map[valueobjects.RootSlug]int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.expense.sum_by_root")
	defer span.End()

	const query = `
		SELECT root_slug, SUM(amount_cents)
		  FROM mecontrola.budgets_expenses
		 WHERE user_id = $1 AND competence = $2 AND deleted_at IS NULL
		 GROUP BY root_slug
	`

	rows, err := db.QueryContext(ctx, query, userID, c.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: sum_by_root: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[valueobjects.RootSlug]int64)
	for rows.Next() {
		var (
			rootSlugStr string
			sumCents    int64
		)
		if scanErr := rows.Scan(&rootSlugStr, &sumCents); scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("budgets/postgres: sum_by_root scan: %w", scanErr)
		}
		slug, slugErr := valueobjects.ParseRootSlug(rootSlugStr)
		if slugErr != nil {
			return nil, fmt.Errorf("budgets/postgres: sum_by_root parse slug: %w", slugErr)
		}
		result[slug] = sumCents
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("budgets/postgres: sum_by_root rows: %w", rowsErr)
	}
	return result, nil
}

func (r *expenseRepository) scanExpenseRow(row database.Row) (entities.Expense, entities.ExpenseTombstone, error) {
	var (
		id                    uuid.UUID
		userID                uuid.UUID
		sourceStr             string
		externalTransactionID string
		subcategoryID         uuid.UUID
		rootSlugStr           string
		competenceStr         string
		amountCents           int64
		occurredAt            time.Time
		version               int64
		tombstoneVersion      sql.NullInt64
		deletedAt             sql.NullTime
		createdAt             time.Time
		updatedAt             time.Time
	)

	err := row.Scan(
		&id, &userID, &sourceStr, &externalTransactionID, &subcategoryID,
		&rootSlugStr, &competenceStr, &amountCents, &occurredAt,
		&version, &tombstoneVersion, &deletedAt, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Expense{}, entities.ExpenseTombstone{}, fmt.Errorf("budgets/postgres: get_by_identity: %w", interfaces.ErrExpenseNotFound)
	}
	if err != nil {
		return entities.Expense{}, entities.ExpenseTombstone{}, fmt.Errorf("budgets/postgres: scan expense: %w", err)
	}

	source, sourceErr := valueobjects.NewProducerSource(sourceStr)
	if sourceErr != nil {
		return entities.Expense{}, entities.ExpenseTombstone{}, fmt.Errorf("budgets/postgres: parse source: %w", sourceErr)
	}

	extID, extErr := valueobjects.NewExternalTransactionID(externalTransactionID)
	if extErr != nil {
		return entities.Expense{}, entities.ExpenseTombstone{}, fmt.Errorf("budgets/postgres: parse external_transaction_id: %w", extErr)
	}

	rootSlug, slugErr := valueobjects.ParseRootSlug(rootSlugStr)
	if slugErr != nil {
		return entities.Expense{}, entities.ExpenseTombstone{}, fmt.Errorf("budgets/postgres: parse root_slug: %w", slugErr)
	}

	competence, compErr := valueobjects.NewCompetence(competenceStr)
	if compErr != nil {
		return entities.Expense{}, entities.ExpenseTombstone{}, fmt.Errorf("budgets/postgres: parse competence: %w", compErr)
	}

	var tombstoneVersionPtr *int64
	if tombstoneVersion.Valid {
		v := tombstoneVersion.Int64
		tombstoneVersionPtr = &v
	}

	var deletedAtPtr *time.Time
	if deletedAt.Valid {
		t := deletedAt.Time
		deletedAtPtr = &t
	}

	expense := entities.HydrateExpense(
		id, userID, source, extID, subcategoryID, rootSlug, competence,
		amountCents, occurredAt, version, tombstoneVersionPtr, deletedAtPtr,
		createdAt, updatedAt,
	)

	if deletedAt.Valid && tombstoneVersion.Valid {
		tombstone := entities.NewExpenseTombstone(
			userID, source, extID,
			tombstoneVersion.Int64, deletedAt.Time,
		)
		return expense, tombstone, nil
	}

	return expense, entities.ExpenseTombstone{}, nil
}

func (r *expenseRepository) PurgeDeleted(ctx context.Context, db database.DBTX, olderThan string, limit int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.expense.purge_deleted")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.budgets_expenses
		 WHERE id IN (
		   SELECT id FROM mecontrola.budgets_expenses
		    WHERE deleted_at < now() - $1::interval
		    LIMIT $2
		 )
	`

	result, err := db.ExecContext(ctx, query, olderThan, limit)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("budgets/postgres: purge_deleted_expenses: %w", err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return 0, fmt.Errorf("budgets/postgres: purge_deleted_expenses rows_affected: %w", rowsErr)
	}
	return n, nil
}
