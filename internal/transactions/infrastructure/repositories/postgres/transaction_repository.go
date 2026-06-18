package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type transactionRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewTransactionRepository(o11y observability.Observability, db database.DBTX) interfaces.TransactionRepository {
	return &transactionRepository{db: db, o11y: o11y}
}

func (r *transactionRepository) Create(ctx context.Context, tx *entities.Transaction) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.create")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.transactions
		       (id, user_id, direction, payment_method, amount_cents, description,
		        category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		        ref_month, occurred_at, version, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	var subID *uuid.UUID
	if v, ok := tx.SubcategoryID().Get(); ok {
		u := v.UUID()
		subID = &u
	}

	_, err := r.db.ExecContext(ctx, query,
		tx.ID(), tx.UserID().UUID(), int(tx.Direction()), int(tx.PaymentMethod()),
		tx.Amount().Cents(), tx.Description().String(),
		tx.CategoryID().UUID(), subID,
		tx.CategoryNameSnapshot(), tx.SubcategoryNameSnapshot(),
		tx.RefMonth().String(), tx.OccurredAt(),
		tx.Version(), tx.DeletedAt(),
		tx.CreatedAt(), tx.UpdatedAt(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: criar lançamento: %w", err)
	}
	return nil
}

func (r *transactionRepository) UpdateWithVersion(ctx context.Context, tx *entities.Transaction, expectedVersion int64) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.update_with_version")
	defer span.End()

	const query = `
		UPDATE mecontrola.transactions
		   SET direction                 = $1,
		       payment_method           = $2,
		       amount_cents             = $3,
		       description              = $4,
		       category_id              = $5,
		       subcategory_id           = $6,
		       category_name_snapshot   = $7,
		       subcategory_name_snapshot = $8,
		       ref_month                = $9,
		       occurred_at              = $10,
		       version                  = $11,
		       updated_at               = $12
		 WHERE id = $13 AND user_id = $14 AND version = $15 AND deleted_at IS NULL
	`

	var subID *uuid.UUID
	if v, ok := tx.SubcategoryID().Get(); ok {
		u := v.UUID()
		subID = &u
	}

	result, err := r.db.ExecContext(ctx, query,
		int(tx.Direction()), int(tx.PaymentMethod()),
		tx.Amount().Cents(), tx.Description().String(),
		tx.CategoryID().UUID(), subID,
		tx.CategoryNameSnapshot(), tx.SubcategoryNameSnapshot(),
		tx.RefMonth().String(), tx.OccurredAt(),
		tx.Version(), tx.UpdatedAt(),
		tx.ID(), tx.UserID().UUID(), expectedVersion,
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.SerializationFailure {
			return fmt.Errorf("transactions/repository: atualizar lançamento: %w", interfaces.ErrTransactionVersionConflict)
		}
		return fmt.Errorf("transactions/repository: atualizar lançamento: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("transactions/repository: atualizar lançamento rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("transactions/repository: atualizar lançamento: %w", interfaces.ErrTransactionVersionConflict)
	}
	return nil
}

func (r *transactionRepository) SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.soft_delete")
	defer span.End()

	const query = `
		UPDATE mecontrola.transactions
		   SET deleted_at = $1,
		       version    = version + 1,
		       updated_at = $2
		 WHERE id = $3 AND user_id = $4 AND version = $5 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, now, now, id, userID, expectedVersion)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: soft-delete: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("transactions/repository: soft-delete rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("transactions/repository: soft-delete: %w", interfaces.ErrTransactionVersionConflict)
	}
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.Transaction, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.get_by_id")
	defer span.End()

	const query = `
		SELECT id, user_id, direction, payment_method, amount_cents, description,
		       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		       ref_month, occurred_at, version, deleted_at, created_at, updated_at
		  FROM mecontrola.transactions
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	rows, err := r.db.QueryContext(ctx, query, id, userID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: buscar lançamento: %w", err)
	}
	defer func() { _ = rows.Close() }()

	txs, err := r.scanRows(rows)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	if len(txs) == 0 {
		return nil, fmt.Errorf("transactions/repository: %w", interfaces.ErrTransactionNotFound)
	}
	return txs[0], nil
}

func (r *transactionRepository) ListByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, cursor interfaces.Cursor, limit int) ([]*entities.Transaction, interfaces.Cursor, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.list_by_month")
	defer span.End()

	var cursorCreatedAt time.Time
	var cursorID uuid.UUID
	if cursor.Value != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(cursor.Value)
		if decErr == nil {
			parts := strings.SplitN(string(decoded), ",", 2)
			if len(parts) == 2 {
				_ = cursorCreatedAt.UnmarshalText([]byte(parts[0]))
				_ = cursorID.UnmarshalText([]byte(parts[1]))
			}
		}
	}

	var query string
	var args []any
	if cursor.Value == "" || cursorCreatedAt.IsZero() {
		query = `
			SELECT id, user_id, direction, payment_method, amount_cents, description,
			       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
			       ref_month, occurred_at, version, deleted_at, created_at, updated_at
			  FROM mecontrola.transactions
			 WHERE user_id = $1 AND ref_month = $2 AND deleted_at IS NULL
			 ORDER BY created_at DESC, id DESC
			 LIMIT $3
		`
		args = []any{userID, refMonth.String(), limit + 1}
	} else {
		query = `
			SELECT id, user_id, direction, payment_method, amount_cents, description,
			       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
			       ref_month, occurred_at, version, deleted_at, created_at, updated_at
			  FROM mecontrola.transactions
			 WHERE user_id = $1 AND ref_month = $2 AND deleted_at IS NULL
			   AND (created_at, id) < ($3, $4)
			 ORDER BY created_at DESC, id DESC
			 LIMIT $5
		`
		args = []any{userID, refMonth.String(), cursorCreatedAt, cursorID, limit + 1}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: listar lançamentos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	txs, err := r.scanRows(rows)
	if err != nil {
		span.RecordError(err)
		return nil, interfaces.Cursor{}, err
	}

	var nextCursor interfaces.Cursor
	if len(txs) > limit {
		last := txs[limit-1]
		tsBytes, _ := last.CreatedAt().UTC().MarshalText()
		idBytes, _ := last.ID().MarshalText()
		raw := string(tsBytes) + "," + string(idBytes)
		nextCursor.Value = base64.StdEncoding.EncodeToString([]byte(raw))
		txs = txs[:limit]
	}

	return txs, nextCursor, nil
}

func (r *transactionRepository) SumByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (int64, int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.sum_by_month")
	defer span.End()

	const query = `
		SELECT
		    COALESCE(SUM(CASE WHEN direction = 1 THEN amount_cents ELSE 0 END), 0) AS income_cents,
		    COALESCE(SUM(CASE WHEN direction = 2 THEN amount_cents ELSE 0 END), 0) AS outcome_cents
		  FROM mecontrola.transactions
		 WHERE user_id = $1 AND ref_month = $2 AND deleted_at IS NULL
	`

	var income, outcome int64
	rows, err := r.db.QueryContext(ctx, query, userID, refMonth.String())
	if err != nil {
		span.RecordError(err)
		return 0, 0, fmt.Errorf("transactions/repository: somar por mês: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if rows.Next() {
		if scanErr := rows.Scan(&income, &outcome); scanErr != nil {
			return 0, 0, fmt.Errorf("transactions/repository: scan somar por mês: %w", scanErr)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("transactions/repository: rows somar: %w", err)
	}
	return income, outcome, nil
}

func (r *transactionRepository) scanRows(rows *sql.Rows) ([]*entities.Transaction, error) {
	var result []*entities.Transaction
	for rows.Next() {
		tx, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("transactions/repository: rows: %w", err)
	}
	return result, nil
}

func (r *transactionRepository) scan(rows *sql.Rows) (*entities.Transaction, error) {
	var (
		id                      uuid.UUID
		userID                  uuid.UUID
		direction               int
		paymentMethod           int
		amountCents             int64
		description             string
		categoryID              uuid.UUID
		subcategoryID           *uuid.UUID
		categoryNameSnapshot    string
		subcategoryNameSnapshot string
		refMonth                string
		occurredAt              time.Time
		version                 int64
		deletedAt               *time.Time
		createdAt               time.Time
		updatedAt               time.Time
	)

	if err := rows.Scan(
		&id, &userID, &direction, &paymentMethod, &amountCents, &description,
		&categoryID, &subcategoryID, &categoryNameSnapshot, &subcategoryNameSnapshot,
		&refMonth, &occurredAt, &version, &deletedAt, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("transactions/repository: scan: %w", err)
	}

	dir, _ := valueobjects.DirectionFromInt(direction)
	pm, _ := valueobjects.PaymentMethodFromInt(paymentMethod)
	amount, _ := valueobjects.NewMoney(amountCents)
	desc, _ := valueobjects.NewDescription(description)
	catID := valueobjects.CategoryIDFromUUID(categoryID)
	rm, _ := valueobjects.NewRefMonth(refMonth)

	var subOpt option.Option[valueobjects.SubcategoryID]
	if subcategoryID != nil {
		subOpt = option.Some(valueobjects.SubcategoryIDFromUUID(*subcategoryID))
	}

	tx := entities.Reconstitute(
		id,
		valueobjects.UserIDFromUUID(userID),
		dir,
		pm,
		amount,
		desc,
		catID,
		subOpt,
		categoryNameSnapshot,
		subcategoryNameSnapshot,
		rm,
		occurredAt,
		version,
		deletedAt,
		createdAt,
		updatedAt,
	)
	return &tx, nil
}
