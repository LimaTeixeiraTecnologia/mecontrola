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

const defaultSearchLimit = 10
const maxSearchLimit = 10

type transactionRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewTransactionRepository(o11y observability.Observability, db database.DBTX) interfaces.TransactionRepository {
	return &transactionRepository{db: db, o11y: o11y}
}

func (r *transactionRepository) Create(ctx context.Context, tx *entities.Transaction) (uuid.UUID, bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.create")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.transactions
		       (id, user_id, direction, payment_method, amount_cents, description,
		        category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		        category_kind, category_path, category_outcome, category_score,
		        category_confidence, category_match_quality, category_signal_type,
		        category_matched_term, category_match_reason, category_decision_source,
		        category_editorial_version, category_decided_at,
		        ref_month, occurred_at, version, deleted_at, created_at, updated_at,
		        origin_wamid, origin_item_seq, origin_operation,
		        card_id, installments_total, card_closing_day, card_due_day)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
		        $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35)
		ON CONFLICT (origin_wamid, origin_item_seq, origin_operation) WHERE origin_wamid IS NOT NULL DO NOTHING
		RETURNING id
	`

	ev := tx.Evidence()
	subID := ev.SubcategoryID()

	var originWamid, originOperation *string
	var originItemSeq *int
	if tx.HasOrigin() {
		w := tx.OriginWamid()
		op := tx.OriginOperation()
		seq := tx.OriginItemSeq()
		originWamid, originOperation, originItemSeq = &w, &op, &seq
	}

	cardID, installmentsTotal, closingDay, dueDay := cardColumns(tx)

	var returnedID uuid.UUID
	err := r.db.QueryRowContext(ctx, query,
		tx.ID(), tx.UserID().UUID(), int(tx.Direction()), int(tx.PaymentMethod()),
		tx.Amount().Cents(), tx.Description().String(),
		tx.CategoryID().UUID(), subID,
		tx.CategoryNameSnapshot(), tx.SubcategoryNameSnapshot(),
		ev.Kind(), ev.Path(), ev.Outcome(), ev.Score(),
		ev.Confidence(), ev.Quality(), ev.SignalType(),
		ev.MatchedTerm(), ev.MatchReason(), ev.Source().String(),
		ev.EditorialVersion(), ev.DecidedAt(),
		tx.RefMonth().String(), tx.OccurredAt(),
		tx.Version(), tx.DeletedAt(),
		tx.CreatedAt(), tx.UpdatedAt(),
		originWamid, originItemSeq, originOperation,
		cardID, installmentsTotal, closingDay, dueDay,
	).Scan(&returnedID)
	if errors.Is(err, sql.ErrNoRows) {
		const existingQuery = `
			SELECT id FROM mecontrola.transactions
			 WHERE origin_wamid = $1 AND origin_item_seq = $2 AND origin_operation = $3
		`
		var existingID uuid.UUID
		if selErr := r.db.QueryRowContext(ctx, existingQuery, originWamid, originItemSeq, originOperation).Scan(&existingID); selErr != nil {
			span.RecordError(selErr)
			return uuid.Nil, false, fmt.Errorf("transactions/repository: reconciliar lançamento: %w", selErr)
		}
		return existingID, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return uuid.Nil, false, fmt.Errorf("transactions/repository: criar lançamento: %w", err)
	}
	return returnedID, true, nil
}

func (r *transactionRepository) UpdateWithVersion(ctx context.Context, tx *entities.Transaction, expectedVersion int64) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.update_with_version")
	defer span.End()

	const query = `
		UPDATE mecontrola.transactions
		   SET direction                  = $1,
		       payment_method             = $2,
		       amount_cents               = $3,
		       description                = $4,
		       category_id                = $5,
		       subcategory_id             = $6,
		       category_name_snapshot     = $7,
		       subcategory_name_snapshot  = $8,
		       category_kind              = $9,
		       category_path              = $10,
		       category_outcome           = $11,
		       category_score             = $12,
		       category_confidence        = $13,
		       category_match_quality     = $14,
		       category_signal_type       = $15,
		       category_matched_term      = $16,
		       category_match_reason      = $17,
		       category_decision_source   = $18,
		       category_editorial_version = $19,
		       category_decided_at        = $20,
		       ref_month                  = $21,
		       occurred_at                = $22,
		       version                    = $23,
		       updated_at                 = $24,
		       card_id                    = $28,
		       installments_total         = $29,
		       card_closing_day           = $30,
		       card_due_day               = $31
		 WHERE id = $25 AND user_id = $26 AND version = $27 AND deleted_at IS NULL
	`

	ev := tx.Evidence()
	subID := ev.SubcategoryID()

	cardID, installmentsTotal, closingDay, dueDay := cardColumns(tx)

	result, err := r.db.ExecContext(ctx, query,
		int(tx.Direction()), int(tx.PaymentMethod()),
		tx.Amount().Cents(), tx.Description().String(),
		tx.CategoryID().UUID(), subID,
		tx.CategoryNameSnapshot(), tx.SubcategoryNameSnapshot(),
		ev.Kind(), ev.Path(), ev.Outcome(), ev.Score(),
		ev.Confidence(), ev.Quality(), ev.SignalType(),
		ev.MatchedTerm(), ev.MatchReason(), ev.Source().String(),
		ev.EditorialVersion(), ev.DecidedAt(),
		tx.RefMonth().String(), tx.OccurredAt(),
		tx.Version(), tx.UpdatedAt(),
		tx.ID(), tx.UserID().UUID(), expectedVersion,
		cardID, installmentsTotal, closingDay, dueDay,
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
		       category_kind, category_path, category_outcome, category_score,
		       category_confidence, category_match_quality, category_signal_type,
		       category_matched_term, category_match_reason, category_decision_source,
		       category_editorial_version, category_decided_at,
		       ref_month, occurred_at, version, deleted_at, created_at, updated_at,
		       card_id, installments_total, card_closing_day, card_due_day
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
			       category_kind, category_path, category_outcome, category_score,
			       category_confidence, category_match_quality, category_signal_type,
			       category_matched_term, category_match_reason, category_decision_source,
			       category_editorial_version, category_decided_at,
			       ref_month, occurred_at, version, deleted_at, created_at, updated_at,
			       card_id, installments_total, card_closing_day, card_due_day
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
			       category_kind, category_path, category_outcome, category_score,
			       category_confidence, category_match_quality, category_signal_type,
			       category_matched_term, category_match_reason, category_decision_source,
			       category_editorial_version, category_decided_at,
			       ref_month, occurred_at, version, deleted_at, created_at, updated_at,
			       card_id, installments_total, card_closing_day, card_due_day
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

func (r *transactionRepository) SearchByDescription(ctx context.Context, userID uuid.UUID, q valueobjects.SearchQuery, refMonth option.Option[valueobjects.RefMonth], limit int) ([]*entities.Transaction, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.search_by_description")
	defer span.End()

	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	var query string
	var args []any
	if rm, ok := refMonth.Get(); ok {
		query = `
			SELECT id, user_id, direction, payment_method, amount_cents, description,
			       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
			       category_kind, category_path, category_outcome, category_score,
			       category_confidence, category_match_quality, category_signal_type,
			       category_matched_term, category_match_reason, category_decision_source,
			       category_editorial_version, category_decided_at,
			       ref_month, occurred_at, version, deleted_at, created_at, updated_at,
			       card_id, installments_total, card_closing_day, card_due_day
			  FROM mecontrola.transactions
			 WHERE user_id = $1 AND deleted_at IS NULL
			   AND ref_month = $2
			   AND description ILIKE '%' || $3 || '%'
			 ORDER BY created_at DESC
			 LIMIT $4
		`
		args = []any{userID, rm.String(), q.String(), limit}
	} else {
		query = `
			SELECT id, user_id, direction, payment_method, amount_cents, description,
			       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
			       category_kind, category_path, category_outcome, category_score,
			       category_confidence, category_match_quality, category_signal_type,
			       category_matched_term, category_match_reason, category_decision_source,
			       category_editorial_version, category_decided_at,
			       ref_month, occurred_at, version, deleted_at, created_at, updated_at,
			       card_id, installments_total, card_closing_day, card_due_day
			  FROM mecontrola.transactions
			 WHERE user_id = $1 AND deleted_at IS NULL
			   AND description ILIKE '%' || $2 || '%'
			 ORDER BY created_at DESC
			 LIMIT $3
		`
		args = []any{userID, q.String(), limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: buscar por descrição: %w", err)
	}
	defer func() { _ = rows.Close() }()

	txs, err := r.scanRows(rows)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	return txs, nil
}

func (r *transactionRepository) SumByMonthExcludingCredit(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (int64, int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.sum_by_month_excluding_credit")
	defer span.End()

	const query = `
		SELECT
		    COALESCE(SUM(CASE WHEN direction = 1 THEN amount_cents ELSE 0 END), 0) AS income_cents,
		    COALESCE(SUM(CASE WHEN direction = 2 THEN amount_cents ELSE 0 END), 0) AS outcome_cents
		  FROM mecontrola.transactions
		 WHERE user_id = $1 AND ref_month = $2 AND payment_method <> 7 AND deleted_at IS NULL
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
		id                       uuid.UUID
		userID                   uuid.UUID
		direction                int
		paymentMethod            int
		amountCents              int64
		description              string
		categoryID               uuid.UUID
		subcategoryID            uuid.UUID
		categoryNameSnapshot     string
		subcategoryNameSnapshot  string
		categoryKind             string
		categoryPath             string
		categoryOutcome          string
		categoryScore            float64
		categoryConfidence       string
		categoryMatchQuality     string
		categorySignalType       string
		categoryMatchedTerm      string
		categoryMatchReason      string
		categoryDecisionSource   string
		categoryEditorialVersion int64
		categoryDecidedAt        time.Time
		refMonth                 string
		occurredAt               time.Time
		version                  int64
		deletedAt                *time.Time
		createdAt                time.Time
		updatedAt                time.Time
		cardID                   *uuid.UUID
		installmentsTotal        *int
		cardClosingDay           *int
		cardDueDay               *int
	)

	if err := rows.Scan(
		&id, &userID, &direction, &paymentMethod, &amountCents, &description,
		&categoryID, &subcategoryID, &categoryNameSnapshot, &subcategoryNameSnapshot,
		&categoryKind, &categoryPath, &categoryOutcome, &categoryScore,
		&categoryConfidence, &categoryMatchQuality, &categorySignalType,
		&categoryMatchedTerm, &categoryMatchReason, &categoryDecisionSource,
		&categoryEditorialVersion, &categoryDecidedAt,
		&refMonth, &occurredAt, &version, &deletedAt, &createdAt, &updatedAt,
		&cardID, &installmentsTotal, &cardClosingDay, &cardDueDay,
	); err != nil {
		return nil, fmt.Errorf("transactions/repository: scan: %w", err)
	}

	dir, _ := valueobjects.DirectionFromInt(direction)
	pm, _ := valueobjects.PaymentMethodFromInt(paymentMethod)
	amount, _ := valueobjects.NewMoney(amountCents)
	desc, _ := valueobjects.NewDescription(description)
	catID := valueobjects.CategoryIDFromUUID(categoryID)
	rm, _ := valueobjects.NewRefMonth(refMonth)

	subOpt := option.Some(valueobjects.SubcategoryIDFromUUID(subcategoryID))

	src, _ := valueobjects.ParseCategoryDecisionSource(categoryDecisionSource)
	evidence := valueobjects.ReconstituteEvidence(
		categoryID,
		subcategoryID,
		categoryKind,
		categoryPath,
		categoryOutcome,
		categoryScore,
		categoryConfidence,
		categoryMatchQuality,
		categorySignalType,
		categoryMatchedTerm,
		categoryMatchReason,
		src,
		categoryEditorialVersion,
		categoryDecidedAt,
	)

	var cardOpt option.Option[valueobjects.CardID]
	if cardID != nil {
		cardOpt = option.Some(valueobjects.CardIDFromUUID(*cardID))
	}

	var installmentsOpt option.Option[valueobjects.InstallmentCount]
	if installmentsTotal != nil {
		if ic, icErr := valueobjects.NewInstallmentCount(*installmentsTotal); icErr == nil {
			installmentsOpt = option.Some(ic)
		}
	}

	var billingOpt option.Option[valueobjects.CardBillingSnapshot]
	if cardClosingDay != nil && cardDueDay != nil {
		if snap, snapErr := valueobjects.NewCardBillingSnapshot(*cardClosingDay, *cardDueDay); snapErr == nil {
			billingOpt = option.Some(snap)
		}
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
		evidence,
		rm,
		occurredAt,
		cardOpt,
		installmentsOpt,
		billingOpt,
		version,
		deletedAt,
		createdAt,
		updatedAt,
	)
	return &tx, nil
}

func cardColumns(tx *entities.Transaction) (*uuid.UUID, *int, *int, *int) {
	var cardID *uuid.UUID
	if v, ok := tx.CardID().Get(); ok {
		u := v.UUID()
		cardID = &u
	}

	var installmentsTotal *int
	if v, ok := tx.InstallmentsTotal().Get(); ok {
		n := v.Value()
		installmentsTotal = &n
	}

	var closingDay, dueDay *int
	if v, ok := tx.BillingSnapshot().Get(); ok {
		c := v.ClosingDay().Value()
		d := v.DueDay().Value()
		closingDay = &c
		dueDay = &d
	}

	return cardID, installmentsTotal, closingDay, dueDay
}

func (r *transactionRepository) GetItemsByTransactionID(ctx context.Context, txID uuid.UUID) ([]*entities.CardInvoiceItem, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.get_items_by_transaction_id")
	defer span.End()

	const query = `
		SELECT id, invoice_id, transaction_id, user_id, ref_month, installment_index, amount_cents, created_at, updated_at
		  FROM mecontrola.transactions_card_invoice_items
		 WHERE transaction_id = $1 AND deleted_at IS NULL
		 ORDER BY installment_index
	`

	rows, err := r.db.QueryContext(ctx, query, txID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: listar itens por transação: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*entities.CardInvoiceItem
	for rows.Next() {
		item, scanErr := scanCardInvoiceItem(rows)
		if scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("transactions/repository: scan item por transação: %w", scanErr)
		}
		items = append(items, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("transactions/repository: rows itens por transação: %w", rowsErr)
	}
	return items, nil
}

func (r *transactionRepository) ReplaceItems(ctx context.Context, txID uuid.UUID, items []*entities.CardInvoiceItem) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.replace_items")
	defer span.End()

	now := time.Now().UTC()
	const deleteQuery = `
		UPDATE mecontrola.transactions_card_invoice_items
		   SET deleted_at = $1, updated_at = $1
		 WHERE transaction_id = $2 AND deleted_at IS NULL
	`
	if _, err := r.db.ExecContext(ctx, deleteQuery, now, txID); err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: remover itens: %w", err)
	}

	const insertQuery = `
		INSERT INTO mecontrola.transactions_card_invoice_items
			(id, invoice_id, transaction_id, user_id, ref_month,
			 installment_index, amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (transaction_id, installment_index) DO UPDATE
		   SET invoice_id = $2, amount_cents = $7, deleted_at = NULL, updated_at = $9
	`
	for _, item := range items {
		_, err := r.db.ExecContext(ctx, insertQuery,
			item.ID(), item.InvoiceID(), txID, item.UserID().UUID(),
			item.RefMonth().String(), item.InstallmentIndex(), item.Amount().Cents(),
			item.CreatedAt(), now,
		)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("transactions/repository: inserir item [%d]: %w", item.InstallmentIndex(), err)
		}
	}
	return nil
}

func (r *transactionRepository) ExistsActiveCreditByCard(ctx context.Context, cardID, userID uuid.UUID) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.transaction.exists_active_credit_by_card")
	defer span.End()

	const query = `
		SELECT EXISTS(
			SELECT 1 FROM mecontrola.transactions
			 WHERE card_id = $1 AND user_id = $2 AND payment_method = 7 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, cardID, userID).Scan(&exists); err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("transactions/repository: verificar crédito ativo por cartão: %w", err)
	}
	return exists, nil
}
