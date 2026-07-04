//go:build e2e

package transactions_e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type outboxEventAdapter struct {
	env outbox.Envelope
}

func (a outboxEventAdapter) GetEventType() string { return a.env.EventType }
func (a outboxEventAdapter) GetPayload() any      { return a.env }

func countOutboxByEventType(ctx context.Context, db *sqlx.DB, eventType, aggregateID string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.outbox_events
		 WHERE event_type = $1
		   AND aggregate_id = $2
	`, eventType, aggregateID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("countOutboxByEventType: %w", err)
	}
	return n, nil
}

func countTransactionsForUser(ctx context.Context, db *sqlx.DB, userID uuid.UUID, refMonth string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.transactions
		 WHERE user_id = $1
		   AND ref_month = $2
		   AND deleted_at IS NULL
	`, userID.String(), refMonth).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("countTransactionsForUser: %w", err)
	}
	return n, nil
}

func fetchTransactionAmountCents(ctx context.Context, db *sqlx.DB, txID string) (int64, error) {
	var amount int64
	err := db.QueryRowContext(ctx, `
		SELECT amount_cents FROM mecontrola.transactions WHERE id = $1
	`, txID).Scan(&amount)
	if err != nil {
		return 0, fmt.Errorf("fetchTransactionAmountCents %s: %w", txID, err)
	}
	return amount, nil
}

func isTransactionSoftDeleted(ctx context.Context, db *sqlx.DB, txID string) (bool, error) {
	var deletedAt *time.Time
	err := db.QueryRowContext(ctx, `
		SELECT deleted_at FROM mecontrola.transactions WHERE id = $1
	`, txID).Scan(&deletedAt)
	if err != nil {
		return false, fmt.Errorf("isTransactionSoftDeleted %s: %w", txID, err)
	}
	return deletedAt != nil, nil
}

func countCardInvoiceItemsForTransaction(ctx context.Context, db *sqlx.DB, transactionID string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.transactions_card_invoice_items
		 WHERE transaction_id = $1
	`, transactionID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("countCardInvoiceItemsForTransaction: %w", err)
	}
	return n, nil
}

func countRecurringTemplatesForUser(ctx context.Context, db *sqlx.DB, userID uuid.UUID) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.transactions_recurring_templates
		 WHERE user_id = $1
		   AND deleted_at IS NULL
	`, userID.String()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("countRecurringTemplatesForUser: %w", err)
	}
	return n, nil
}

func isRecurringTemplateSoftDeleted(ctx context.Context, db *sqlx.DB, rtID string) (bool, error) {
	var deletedAt *time.Time
	err := db.QueryRowContext(ctx, `
		SELECT deleted_at FROM mecontrola.transactions_recurring_templates WHERE id = $1
	`, rtID).Scan(&deletedAt)
	if err != nil {
		return false, fmt.Errorf("isRecurringTemplateSoftDeleted %s: %w", rtID, err)
	}
	return deletedAt != nil, nil
}

func countMonthlySummaryRows(ctx context.Context, db *sqlx.DB, userID uuid.UUID, refMonth string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.transactions_monthly_summary
		 WHERE user_id = $1
		   AND ref_month = $2
	`, userID.String(), refMonth).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("countMonthlySummaryRows: %w", err)
	}
	return n, nil
}

func fetchMonthlySummaryAmountCents(ctx context.Context, db *sqlx.DB, userID uuid.UUID, refMonth string) (income, outcome int64, err error) {
	err = db.QueryRowContext(ctx, `
		SELECT income_cents, outcome_cents
		  FROM mecontrola.transactions_monthly_summary
		 WHERE user_id = $1
		   AND ref_month = $2
	`, userID.String(), refMonth).Scan(&income, &outcome)
	if err != nil {
		return 0, 0, fmt.Errorf("fetchMonthlySummaryAmountCents: %w", err)
	}
	return income, outcome, nil
}

func countRecurringMaterializationsForDay(ctx context.Context, db *sqlx.DB, templateID, refMonth string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM mecontrola.transactions_recurring_materializations
		 WHERE template_id = $1
		   AND ref_month = $2
	`, templateID, refMonth).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("countRecurringMaterializationsForDay: %w", err)
	}
	return n, nil
}

func insertCardViaSQL(ctx context.Context, db *sqlx.DB, userID uuid.UUID, bank, nickname string, closingDay, dueDay int) (string, error) {
	id := uuid.NewString()
	_, err := db.ExecContext(ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
	`, id, userID.String(), bank, nickname, closingDay, dueDay)
	if err != nil {
		return "", fmt.Errorf("insertCardViaSQL: %w", err)
	}
	return id, nil
}

func drainOutboxToConsumer(ctx context.Context, db *sqlx.DB, consumer interface {
	Handle(ctx context.Context, event events.Event) error
	Stop(ctx context.Context)
}, debounce time.Duration) error {
	storage := outbox.NewPostgresStorage(db)
	for {
		rows, err := storage.ClaimBatch(ctx, "e2e-consumer-drain", 100)
		if err != nil {
			return fmt.Errorf("drainOutboxToConsumer ClaimBatch: %w", err)
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			env := outbox.Pack(row)
			if handleErr := consumer.Handle(ctx, outboxEventAdapter{env: env}); handleErr != nil {
				return fmt.Errorf("drainOutboxToConsumer Handle: %w", handleErr)
			}
			if markErr := storage.MarkPublished(ctx, row.ID); markErr != nil {
				return fmt.Errorf("drainOutboxToConsumer MarkPublished: %w", markErr)
			}
		}
	}

	time.Sleep(debounce + 200*time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	consumer.Stop(stopCtx)

	return nil
}
