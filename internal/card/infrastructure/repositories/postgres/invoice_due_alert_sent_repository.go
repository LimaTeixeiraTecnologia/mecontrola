package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
)

type invoiceDueAlertSentRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewInvoiceDueAlertSentRepository(o11y observability.Observability, db database.DBTX) interfaces.InvoiceDueAlertSentRepository {
	return &invoiceDueAlertSentRepository{o11y: o11y, db: db}
}

func (r *invoiceDueAlertSentRepository) ListSentForDueDates(ctx context.Context, dueDates []time.Time) ([]interfaces.InvoiceDueAlertSentRecord, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.invoice_due_alert_sent.list_for_due_dates")
	defer span.End()

	if len(dueDates) == 0 {
		return nil, nil
	}

	days := make([]time.Time, 0, len(dueDates))
	for _, d := range dueDates {
		days = append(days, d.UTC().Truncate(24*time.Hour))
	}

	const query = `
		SELECT user_id, card_id, ref_due_date, notified_at
		  FROM mecontrola.card_invoice_alerts_sent
		 WHERE ref_due_date = ANY($1::date[])
	`

	rows, err := r.db.QueryContext(ctx, query, days)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.list_for_due_dates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []interfaces.InvoiceDueAlertSentRecord
	for rows.Next() {
		var (
			userID     uuid.UUID
			cardID     uuid.UUID
			refDueDate time.Time
			notifiedAt sql.NullTime
		)
		if scanErr := rows.Scan(&userID, &cardID, &refDueDate, &notifiedAt); scanErr != nil {
			return nil, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.list_for_due_dates scan: %w", scanErr)
		}
		rec := interfaces.InvoiceDueAlertSentRecord{
			UserID:     userID,
			CardID:     cardID,
			RefDueDate: refDueDate.UTC(),
		}
		if notifiedAt.Valid {
			rec.NotifiedAt = notifiedAt.Time.UTC()
		}
		out = append(out, rec)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.list_for_due_dates rows: %w", rowsErr)
	}
	return out, nil
}

func (r *invoiceDueAlertSentRepository) InsertSent(ctx context.Context, userID, cardID uuid.UUID, refDueDate time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.invoice_due_alert_sent.insert_sent")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.card_invoice_alerts_sent (user_id, card_id, ref_due_date)
		VALUES ($1, $2, $3::date)
		ON CONFLICT (user_id, card_id, ref_due_date) DO NOTHING
	`

	day := refDueDate.UTC().Truncate(24 * time.Hour)
	if _, err := r.db.ExecContext(ctx, query, userID, cardID, day); err != nil {
		span.RecordError(err)
		return fmt.Errorf("card.repository.pg: invoice_due_alert_sent.insert_sent: %w", err)
	}
	return nil
}

func (r *invoiceDueAlertSentRepository) IsNotified(ctx context.Context, userID, cardID uuid.UUID, refDueDate time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.invoice_due_alert_sent.is_notified")
	defer span.End()

	day := refDueDate.UTC().Truncate(24 * time.Hour)
	const query = `
		SELECT notified_at IS NOT NULL
		  FROM mecontrola.card_invoice_alerts_sent
		 WHERE user_id = $1 AND card_id = $2 AND ref_due_date = $3::date
		 LIMIT 1
	`
	var notified bool
	err := r.db.QueryRowContext(ctx, query, userID, cardID, day).Scan(&notified)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.is_notified: %w", interfaces.ErrInvoiceDueAlertRecordMissing)
	}
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.is_notified: %w", err)
	}
	return notified, nil
}

func (r *invoiceDueAlertSentRepository) MarkNotified(ctx context.Context, userID, cardID uuid.UUID, refDueDate time.Time, channel string, notifiedAt time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.invoice_due_alert_sent.mark_notified")
	defer span.End()

	day := refDueDate.UTC().Truncate(24 * time.Hour)
	const query = `
		UPDATE mecontrola.card_invoice_alerts_sent
		   SET notified_at = $4,
		       notify_channel = $5
		 WHERE user_id = $1 AND card_id = $2 AND ref_due_date = $3::date
		   AND notified_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, userID, cardID, day, notifiedAt.UTC(), channel)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.mark_notified: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("card.repository.pg: invoice_due_alert_sent.mark_notified rows affected: %w", err)
	}
	return rowsAffected > 0, nil
}
