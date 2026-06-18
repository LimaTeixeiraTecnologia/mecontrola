package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type monthlySummaryRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewMonthlySummaryRepository(o11y observability.Observability, db database.DBTX) interfaces.MonthlySummaryRepository {
	return &monthlySummaryRepository{db: db, o11y: o11y}
}

func (r *monthlySummaryRepository) Upsert(
	ctx context.Context,
	userID uuid.UUID,
	refMonth valueobjects.RefMonth,
	incomeCents, outcomeCents int64,
	updatedAt time.Time,
) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.monthly_summary.upsert")
	defer span.End()

	totalCents := incomeCents - outcomeCents
	const q = `
		INSERT INTO mecontrola.transactions_monthly_summary
		       (user_id, ref_month, income_cents, outcome_cents, total_cents, version, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6)
		ON CONFLICT (user_id, ref_month) DO UPDATE
		   SET income_cents  = EXCLUDED.income_cents,
		       outcome_cents = EXCLUDED.outcome_cents,
		       total_cents   = EXCLUDED.total_cents,
		       version       = mecontrola.transactions_monthly_summary.version + 1,
		       updated_at    = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, q, userID, refMonth.String(), incomeCents, outcomeCents, totalCents, updatedAt)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: upsert resumo mensal: %w", err)
	}
	return nil
}

func (r *monthlySummaryRepository) Get(
	ctx context.Context,
	userID uuid.UUID,
	refMonth valueobjects.RefMonth,
) (*entities.MonthlySummary, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.monthly_summary.get")
	defer span.End()

	const q = `
		SELECT user_id, ref_month, income_cents, outcome_cents, version, updated_at
		  FROM mecontrola.transactions_monthly_summary
		 WHERE user_id = $1 AND ref_month = $2
	`
	rows, err := r.db.QueryContext(ctx, q, userID, refMonth.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: buscar resumo mensal: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("transactions/repository: buscar resumo mensal rows: %w", err)
		}
		return nil, nil
	}

	var (
		uid          uuid.UUID
		refMonthStr  string
		incomeCents  int64
		outcomeCents int64
		version      int64
		updatedAt    time.Time
	)
	if err := rows.Scan(&uid, &refMonthStr, &incomeCents, &outcomeCents, &version, &updatedAt); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: scan resumo mensal: %w", err)
	}

	rm, err := valueobjects.NewRefMonth(refMonthStr)
	if err != nil {
		return nil, fmt.Errorf("transactions/repository: parse ref_month resumo: %w", err)
	}

	summary := entities.NewMonthlySummary(uid, rm, incomeCents, outcomeCents, version, &updatedAt)
	return &summary, nil
}

func (r *monthlySummaryRepository) ListActiveSince(
	ctx context.Context,
	since time.Time,
	cursor interfaces.Cursor,
	batchSize int,
) ([]interfaces.MonthlySummaryKey, interfaces.Cursor, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.monthly_summary.list_active_since")
	defer span.End()

	q, args := buildActiveSinceQuery(since, cursor, batchSize)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		span.RecordError(err)
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: listar resumos ativos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	keys, scanErr := scanSummaryKeys(rows)
	if scanErr != nil {
		return nil, interfaces.Cursor{}, scanErr
	}

	var nextCursor interfaces.Cursor
	if len(keys) > batchSize {
		last := keys[batchSize-1]
		uidBytes, _ := last.UserID.MarshalText()
		raw := string(uidBytes) + "," + last.RefMonth
		nextCursor.Value = base64.StdEncoding.EncodeToString([]byte(raw))
		keys = keys[:batchSize]
	}
	return keys, nextCursor, nil
}

func buildActiveSinceQuery(since time.Time, cursor interfaces.Cursor, batchSize int) (string, []any) {
	var cursorUserID uuid.UUID
	var cursorRefMonth string
	if cursor.Value != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(cursor.Value)
		if decErr == nil {
			parts := strings.SplitN(string(decoded), ",", 2)
			if len(parts) == 2 {
				_ = cursorUserID.UnmarshalText([]byte(parts[0]))
				cursorRefMonth = parts[1]
			}
		}
	}
	if cursor.Value == "" || cursorRefMonth == "" {
		q := `SELECT user_id, ref_month FROM mecontrola.transactions_monthly_summary WHERE updated_at >= $1 ORDER BY user_id ASC, ref_month ASC LIMIT $2`
		return q, []any{since, batchSize + 1}
	}
	q := `SELECT user_id, ref_month FROM mecontrola.transactions_monthly_summary WHERE updated_at >= $1 AND (user_id, ref_month) > ($2, $3) ORDER BY user_id ASC, ref_month ASC LIMIT $4`
	return q, []any{since, cursorUserID, cursorRefMonth, batchSize + 1}
}

func scanSummaryKeys(rows *sql.Rows) ([]interfaces.MonthlySummaryKey, error) {
	var keys []interfaces.MonthlySummaryKey
	for rows.Next() {
		var uid uuid.UUID
		var rm string
		if scanErr := rows.Scan(&uid, &rm); scanErr != nil {
			return nil, fmt.Errorf("transactions/repository: scan resumos ativos: %w", scanErr)
		}
		keys = append(keys, interfaces.MonthlySummaryKey{UserID: uid, RefMonth: rm})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("transactions/repository: rows resumos ativos: %w", err)
	}
	return keys, nil
}

func (r *monthlySummaryRepository) ListEntries(
	ctx context.Context,
	userID uuid.UUID,
	refMonth valueobjects.RefMonth,
	cursor interfaces.Cursor,
	limit int,
) ([]interfaces.MonthlyEntry, interfaces.Cursor, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.monthly_summary.list_entries")
	defer span.End()

	q, args := buildEntriesQuery(userID, refMonth, cursor, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		span.RecordError(err)
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: listar entradas mensais: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries, scanErr := scanMonthlyEntries(rows)
	if scanErr != nil {
		span.RecordError(scanErr)
		return nil, interfaces.Cursor{}, scanErr
	}

	var nextCursor interfaces.Cursor
	if len(entries) > limit {
		last := entries[limit-1]
		tsBytes, _ := last.CreatedAt.UTC().MarshalText()
		raw := string(tsBytes) + "," + last.ID
		nextCursor.Value = base64.StdEncoding.EncodeToString([]byte(raw))
		entries = entries[:limit]
	}
	return entries, nextCursor, nil
}

func buildEntriesQuery(userID uuid.UUID, refMonth valueobjects.RefMonth, cursor interfaces.Cursor, limit int) (string, []any) {
	var cursorCreatedAt time.Time
	var cursorID string
	if cursor.Value != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(cursor.Value)
		if decErr == nil {
			parts := strings.SplitN(string(decoded), ",", 2)
			if len(parts) == 2 {
				_ = cursorCreatedAt.UnmarshalText([]byte(parts[0]))
				cursorID = parts[1]
			}
		}
	}

	if cursor.Value == "" || cursorCreatedAt.IsZero() {
		q := `
			SELECT 'transaction' AS kind, id::text, user_id, ref_month, amount_cents, direction, description, created_at
			  FROM mecontrola.transactions
			 WHERE user_id = $1 AND ref_month = $2 AND deleted_at IS NULL
			UNION ALL
			SELECT 'card_invoice_item', i.id::text, i.user_id, i.ref_month, i.amount_cents, 2, '', i.created_at
			  FROM mecontrola.transactions_card_invoice_items i
			 WHERE i.user_id = $1 AND i.ref_month = $2 AND i.deleted_at IS NULL
			 ORDER BY created_at DESC, id DESC LIMIT $3
		`
		return q, []any{userID, refMonth.String(), limit + 1}
	}

	q := `
		SELECT 'transaction' AS kind, id::text, user_id, ref_month, amount_cents, direction, description, created_at
		  FROM mecontrola.transactions
		 WHERE user_id = $1 AND ref_month = $2 AND deleted_at IS NULL AND (created_at, id::text) < ($3, $4)
		UNION ALL
		SELECT 'card_invoice_item', i.id::text, i.user_id, i.ref_month, i.amount_cents, 2, '', i.created_at
		  FROM mecontrola.transactions_card_invoice_items i
		 WHERE i.user_id = $1 AND i.ref_month = $2 AND i.deleted_at IS NULL AND (i.created_at, i.id::text) < ($3, $4)
		 ORDER BY created_at DESC, id DESC LIMIT $5
	`
	return q, []any{userID, refMonth.String(), cursorCreatedAt, cursorID, limit + 1}
}

func scanMonthlyEntries(rows *sql.Rows) ([]interfaces.MonthlyEntry, error) {
	var entries []interfaces.MonthlyEntry
	for rows.Next() {
		var (
			kind        string
			id          string
			uid         uuid.UUID
			rm          string
			amountCents int64
			direction   int
			description string
			createdAt   time.Time
		)
		if scanErr := rows.Scan(&kind, &id, &uid, &rm, &amountCents, &direction, &description, &createdAt); scanErr != nil {
			return nil, fmt.Errorf("transactions/repository: scan entradas mensais: %w", scanErr)
		}
		dirStr := "outcome"
		if direction == 1 {
			dirStr = "income"
		}
		entries = append(entries, interfaces.MonthlyEntry{
			Kind:        kind,
			ID:          id,
			UserID:      uid,
			RefMonth:    rm,
			AmountCents: amountCents,
			Direction:   dirStr,
			Description: description,
			CreatedAt:   createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("transactions/repository: rows entradas mensais: %w", err)
	}
	return entries, nil
}
