package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrCardInvoiceNotFound = errors.New("transactions: fatura não encontrada")
var ErrCardInvoiceVersionConflict = errors.New("transactions: conflito de versão na fatura")

type cardInvoiceRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewCardInvoiceRepository(o11y observability.Observability, db database.DBTX) interfaces.CardInvoiceRepository {
	return &cardInvoiceRepository{db: db, o11y: o11y}
}

func (r *cardInvoiceRepository) UpsertByMonth(
	ctx context.Context,
	userID, cardID uuid.UUID,
	refMonth valueobjects.RefMonth,
	closingAt, dueAt time.Time,
) (*entities.CardInvoice, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_invoice.upsert_by_month")
	defer span.End()

	now := time.Now().UTC()
	newID := uuid.New()

	const q = `
		INSERT INTO mecontrola.transactions_card_invoices
			(id, user_id, card_id, ref_month, closing_at, due_at, items_total_cents, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,0,1,$7,$7)
		ON CONFLICT (user_id, card_id, ref_month) DO UPDATE
		   SET closing_at=$5, due_at=$6, updated_at=$7
		RETURNING id, user_id, card_id, ref_month, closing_at, due_at, items_total_cents, version, created_at, updated_at
	`
	row := r.db.QueryRowContext(ctx, q, newID, userID, cardID, refMonth.String(), closingAt, dueAt, now)
	return scanCardInvoice(row)
}

func (r *cardInvoiceRepository) ApplyDelta(ctx context.Context, invoiceID uuid.UUID, deltaCents int64, expectedVersion int64) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_invoice.apply_delta")
	defer span.End()

	if deltaCents == 0 {
		return nil
	}

	const q = `
		UPDATE mecontrola.transactions_card_invoices
		   SET items_total_cents = items_total_cents + $1,
		       version           = version + 1,
		       updated_at        = NOW()
		 WHERE id=$2 AND version=$3
	`
	res, err := r.db.ExecContext(ctx, q, deltaCents, invoiceID, expectedVersion)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/postgres: apply delta fatura: %w", err)
	}
	n, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("transactions/postgres: apply delta rows_affected: %w", rowsErr)
	}
	if n == 0 {
		return ErrCardInvoiceVersionConflict
	}
	return nil
}

func (r *cardInvoiceRepository) GetByMonth(
	ctx context.Context,
	userID, cardID uuid.UUID,
	refMonth valueobjects.RefMonth,
) (*entities.CardInvoice, []*entities.CardInvoiceItem, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_invoice.get_by_month")
	defer span.End()

	const invQ = `
		SELECT id, user_id, card_id, ref_month, closing_at, due_at, items_total_cents, version, created_at, updated_at
		  FROM mecontrola.transactions_card_invoices
		 WHERE user_id=$1 AND card_id=$2 AND ref_month=$3
	`
	invRow := r.db.QueryRowContext(ctx, invQ, userID, cardID, refMonth.String())
	inv, err := scanCardInvoice(invRow)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		span.RecordError(err)
		return nil, nil, fmt.Errorf("transactions/postgres: obter fatura: %w", err)
	}

	const itemsQ = `
		SELECT id, invoice_id, transaction_id, user_id, ref_month, installment_index, amount_cents, created_at, updated_at
		  FROM mecontrola.transactions_card_invoice_items
		 WHERE invoice_id=$1 AND deleted_at IS NULL
		 ORDER BY installment_index
	`
	rows, queryErr := r.db.QueryContext(ctx, itemsQ, inv.ID())
	if queryErr != nil {
		span.RecordError(queryErr)
		return nil, nil, fmt.Errorf("transactions/postgres: listar itens da fatura: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()

	var items []*entities.CardInvoiceItem
	for rows.Next() {
		item, scanErr := scanCardInvoiceItem(rows)
		if scanErr != nil {
			return nil, nil, fmt.Errorf("transactions/postgres: scan item: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("transactions/postgres: listar itens iteration: %w", err)
	}

	return inv, items, nil
}

func (r *cardInvoiceRepository) SumByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_invoice.sum_by_month")
	defer span.End()

	const q = `
		SELECT COALESCE(SUM(items_total_cents), 0)
		  FROM mecontrola.transactions_card_invoices
		 WHERE user_id=$1 AND ref_month=$2
	`
	row := r.db.QueryRowContext(ctx, q, userID, refMonth.String())
	var total int64
	if err := row.Scan(&total); err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("transactions/postgres: sum faturas: %w", err)
	}
	return total, nil
}

type invoiceScanner interface {
	Scan(dest ...any) error
}

func scanCardInvoice(s invoiceScanner) (*entities.CardInvoice, error) {
	var (
		id              uuid.UUID
		userID          uuid.UUID
		cardID          uuid.UUID
		refMonthStr     string
		closingAt       time.Time
		dueAt           time.Time
		itemsTotalCents int64
		version         int64
		createdAt       time.Time
		updatedAt       time.Time
	)
	err := s.Scan(&id, &userID, &cardID, &refMonthStr, &closingAt, &dueAt, &itemsTotalCents, &version, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	refMonth, rmErr := valueobjects.NewRefMonth(refMonthStr)
	if rmErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse ref_month: %w", rmErr)
	}

	inv := entities.NewCardInvoice(
		id,
		valueobjects.UserIDFromUUID(userID),
		valueobjects.CardIDFromUUID(cardID),
		refMonth,
		closingAt,
		dueAt,
		createdAt,
	)
	inv.HydrateVersion(version, itemsTotalCents, updatedAt)
	return &inv, nil
}

type itemScanner interface {
	Scan(dest ...any) error
}

func scanCardInvoiceItem(s itemScanner) (*entities.CardInvoiceItem, error) {
	var (
		id               uuid.UUID
		invoiceID        uuid.UUID
		transactionID    uuid.UUID
		userID           uuid.UUID
		refMonthStr      string
		installmentIndex int
		amountCents      int64
		createdAt        time.Time
		updatedAt        time.Time
	)
	err := s.Scan(&id, &invoiceID, &transactionID, &userID, &refMonthStr, &installmentIndex, &amountCents, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	refMonth, rmErr := valueobjects.NewRefMonth(refMonthStr)
	if rmErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse item ref_month: %w", rmErr)
	}

	amount, amtErr := valueobjects.NewMoney(amountCents)
	if amtErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse item amount: %w", amtErr)
	}

	item := entities.NewCardInvoiceItem(
		id, invoiceID, transactionID,
		valueobjects.UserIDFromUUID(userID),
		refMonth, installmentIndex, amount, createdAt,
	)
	return &item, nil
}
