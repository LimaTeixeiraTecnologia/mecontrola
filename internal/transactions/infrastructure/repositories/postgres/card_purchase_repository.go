package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrCardPurchaseNotFound = errors.New("transactions: compra de cartão não encontrada")
var ErrCardPurchaseVersionConflict = errors.New("transactions: conflito de versão na compra de cartão")

type cardPurchaseRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewCardPurchaseRepository(o11y observability.Observability, db database.DBTX) interfaces.CardPurchaseRepository {
	return &cardPurchaseRepository{db: db, o11y: o11y}
}

func (r *cardPurchaseRepository) Create(ctx context.Context, p *entities.CardPurchase) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_purchase.create")
	defer span.End()

	var subID *uuid.UUID
	if sub, ok := p.SubcategoryID().Get(); ok {
		v := sub.UUID()
		subID = &v
	}

	const q = `
		INSERT INTO mecontrola.transactions_card_purchases
			(id, user_id, card_id, direction, total_amount_cents, installments_total,
			 description, category_id, subcategory_id, category_name_snapshot,
			 subcategory_name_snapshot, purchased_at, card_closing_day, card_due_day,
			 version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`
	_, err := r.db.ExecContext(ctx, q,
		p.ID(), p.UserID().UUID(), p.CardID().UUID(), 2,
		p.TotalAmount().Cents(), p.InstallmentsTotal().Value(),
		p.Description().String(), p.CategoryID().UUID(), subID,
		p.CategoryNameSnapshot(), nullableString(p.SubcategoryNameSnapshot()),
		p.PurchasedAt(), p.BillingSnapshot().ClosingDay().Value(), p.BillingSnapshot().DueDay().Value(),
		p.Version(), p.CreatedAt(), p.UpdatedAt(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/postgres: criar compra: %w", err)
	}
	return nil
}

func (r *cardPurchaseRepository) UpdateWithVersion(ctx context.Context, p *entities.CardPurchase, expectedVersion int64) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_purchase.update_with_version")
	defer span.End()

	var subID *uuid.UUID
	if sub, ok := p.SubcategoryID().Get(); ok {
		v := sub.UUID()
		subID = &v
	}

	const q = `
		UPDATE mecontrola.transactions_card_purchases
		   SET total_amount_cents=$1, installments_total=$2, description=$3,
		       category_id=$4, subcategory_id=$5, category_name_snapshot=$6,
		       subcategory_name_snapshot=$7, purchased_at=$8, version=$9, updated_at=$10
		 WHERE id=$11 AND user_id=$12 AND version=$13 AND deleted_at IS NULL
	`
	res, err := r.db.ExecContext(ctx, q,
		p.TotalAmount().Cents(), p.InstallmentsTotal().Value(), p.Description().String(),
		p.CategoryID().UUID(), subID, p.CategoryNameSnapshot(),
		nullableString(p.SubcategoryNameSnapshot()), p.PurchasedAt(),
		p.Version(), p.UpdatedAt(),
		p.ID(), p.UserID().UUID(), expectedVersion,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/postgres: atualizar compra: %w", err)
	}
	n, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("transactions/postgres: atualizar compra rows_affected: %w", rowsErr)
	}
	if n == 0 {
		return ErrCardPurchaseVersionConflict
	}
	return nil
}

func (r *cardPurchaseRepository) SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_purchase.soft_delete")
	defer span.End()

	const q = `
		UPDATE mecontrola.transactions_card_purchases
		   SET deleted_at=$1, version=version+1, updated_at=$1
		 WHERE id=$2 AND user_id=$3 AND version=$4 AND deleted_at IS NULL
	`
	res, err := r.db.ExecContext(ctx, q, now, id, userID, expectedVersion)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/postgres: deletar compra: %w", err)
	}
	n, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("transactions/postgres: deletar compra rows_affected: %w", rowsErr)
	}
	if n == 0 {
		return ErrCardPurchaseVersionConflict
	}
	return nil
}

func (r *cardPurchaseRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.CardPurchase, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_purchase.get_by_id")
	defer span.End()

	const q = `
		SELECT id, user_id, card_id, total_amount_cents, installments_total,
		       description, category_id, subcategory_id, category_name_snapshot,
		       subcategory_name_snapshot, purchased_at, card_closing_day, card_due_day,
		       version, created_at, updated_at
		  FROM mecontrola.transactions_card_purchases
		 WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
	`
	row := r.db.QueryRowContext(ctx, q, id, userID)
	p, err := scanCardPurchase(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardPurchaseNotFound
	}
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/postgres: obter compra: %w", err)
	}
	return p, nil
}

func (r *cardPurchaseRepository) ListByCardAndMonth(
	ctx context.Context,
	userID, cardID uuid.UUID,
	refMonth *valueobjects.RefMonth,
	cursor interfaces.Cursor,
	limit int,
) ([]*entities.CardPurchase, interfaces.Cursor, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_purchase.list_by_card_and_month")
	defer span.End()

	if limit <= 0 {
		limit = 20
	}

	cursorTime, cursorID, err := decodePurchaseCursor(cursor.Value)
	if err != nil {
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/postgres: listar compras cursor inválido: %w", err)
	}

	args := []any{userID, cardID}
	q := `
		SELECT id, user_id, card_id, total_amount_cents, installments_total,
		       description, category_id, subcategory_id, category_name_snapshot,
		       subcategory_name_snapshot, purchased_at, card_closing_day, card_due_day,
		       version, created_at, updated_at
		  FROM mecontrola.transactions_card_purchases
		 WHERE user_id=$1 AND card_id=$2 AND deleted_at IS NULL
	`
	idx := 3

	if refMonth != nil {
		q += fmt.Sprintf(` AND purchased_at >= date_trunc('month', $%d::date) AND purchased_at < date_trunc('month', $%d::date) + interval '1 month'`, idx, idx)
		args = append(args, refMonth.String()+"-01")
		idx++
	}

	if !cursorTime.IsZero() {
		q += fmt.Sprintf(` AND (created_at, id) < ($%d, $%d)`, idx, idx+1)
		args = append(args, cursorTime, cursorID)
		idx += 2
	}

	q += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, idx)
	args = append(args, limit+1)

	rows, queryErr := r.db.QueryContext(ctx, q, args...)
	if queryErr != nil {
		span.RecordError(queryErr)
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/postgres: listar compras: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.CardPurchase
	for rows.Next() {
		p, scanErr := scanCardPurchase(rows)
		if scanErr != nil {
			return nil, interfaces.Cursor{}, fmt.Errorf("transactions/postgres: scan compra: %w", scanErr)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/postgres: listar compras iteration: %w", err)
	}

	var nextCursor interfaces.Cursor
	if len(result) > limit {
		result = result[:limit]
		last := result[len(result)-1]
		nextCursor = interfaces.Cursor{Value: encodePurchaseCursor(last.CreatedAt(), last.ID())}
	}
	return result, nextCursor, nil
}

func (r *cardPurchaseRepository) ReplaceItems(ctx context.Context, purchaseID uuid.UUID, items []*entities.CardInvoiceItem) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.card_purchase.replace_items")
	defer span.End()

	now := time.Now().UTC()
	const deleteQ = `
		UPDATE mecontrola.transactions_card_invoice_items
		   SET deleted_at=$1, updated_at=$1
		 WHERE purchase_id=$2 AND deleted_at IS NULL
	`
	if _, err := r.db.ExecContext(ctx, deleteQ, now, purchaseID); err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/postgres: remover items: %w", err)
	}

	for _, item := range items {
		const insertQ = `
			INSERT INTO mecontrola.transactions_card_invoice_items
				(id, invoice_id, purchase_id, user_id, ref_month,
				 installment_index, amount_cents, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (purchase_id, installment_index) DO UPDATE
			   SET invoice_id=$2, amount_cents=$7, deleted_at=NULL, updated_at=$9
		`
		_, err := r.db.ExecContext(ctx, insertQ,
			item.ID(), item.InvoiceID(), item.PurchaseID(), item.UserID().UUID(),
			item.RefMonth().String(), item.InstallmentIndex(), item.Amount().Cents(),
			item.CreatedAt(), now,
		)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("transactions/postgres: inserir item [%d]: %w", item.InstallmentIndex(), err)
		}
	}
	return nil
}

type purchaseScanner interface {
	Scan(dest ...any) error
}

func scanCardPurchase(s purchaseScanner) (*entities.CardPurchase, error) {
	var (
		id                      uuid.UUID
		userID                  uuid.UUID
		cardID                  uuid.UUID
		totalAmountCents        int64
		installmentsTotal       int
		description             string
		categoryID              uuid.UUID
		subcategoryID           *uuid.UUID
		categoryNameSnapshot    string
		subcategoryNameSnapshot *string
		purchasedAt             time.Time
		closingDay              int
		dueDay                  int
		version                 int64
		createdAt               time.Time
		updatedAt               time.Time
	)

	err := s.Scan(
		&id, &userID, &cardID, &totalAmountCents, &installmentsTotal,
		&description, &categoryID, &subcategoryID, &categoryNameSnapshot,
		&subcategoryNameSnapshot, &purchasedAt, &closingDay, &dueDay,
		&version, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	amount, amtErr := valueobjects.NewMoney(totalAmountCents)
	if amtErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse amount: %w", amtErr)
	}
	inst, instErr := valueobjects.NewInstallmentCount(installmentsTotal)
	if instErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse installments: %w", instErr)
	}
	desc, descErr := valueobjects.NewDescription(description)
	if descErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse description: %w", descErr)
	}
	snap, snapErr := valueobjects.NewCardBillingSnapshot(closingDay, dueDay)
	if snapErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse snapshot: %w", snapErr)
	}
	catID, catErr := valueobjects.ParseCategoryID(categoryID.String())
	if catErr != nil {
		return nil, fmt.Errorf("transactions/postgres: parse category_id: %w", catErr)
	}

	var subOpt option.Option[valueobjects.SubcategoryID]
	if subcategoryID != nil {
		sub, subErr := valueobjects.ParseSubcategoryID(subcategoryID.String())
		if subErr != nil {
			return nil, fmt.Errorf("transactions/postgres: parse subcategory_id: %w", subErr)
		}
		subOpt = option.Some(sub)
	}

	var subName string
	if subcategoryNameSnapshot != nil {
		subName = *subcategoryNameSnapshot
	}

	p := entities.NewCardPurchase(
		id,
		valueobjects.UserIDFromUUID(userID),
		valueobjects.CardIDFromUUID(cardID),
		amount,
		inst,
		desc,
		catID,
		subOpt,
		categoryNameSnapshot,
		subName,
		purchasedAt,
		snap,
		createdAt,
	)
	p.HydrateVersion(version, updatedAt)
	return &p, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func encodePurchaseCursor(t time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%s|%s", t.UTC().Format(time.RFC3339Nano), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodePurchaseCursor(cursor string) (time.Time, uuid.UUID, error) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decode cursor: %w", err)
	}
	parts := splitPurchaseCursor(string(raw))
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor format: expected 2 parts got %d", len(parts))
	}
	t, parseErr := time.Parse(time.RFC3339Nano, parts[0])
	if parseErr != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor time: %w", parseErr)
	}
	id, idErr := uuid.Parse(parts[1])
	if idErr != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor id: %w", idErr)
	}
	return t, id, nil
}

func splitPurchaseCursor(s string) []string {
	const uuidLen = 36
	idx := len(s) - uuidLen
	if idx <= 1 {
		return nil
	}
	if s[idx-1] != '|' {
		return nil
	}
	return []string{s[:idx-1], s[idx:]}
}
