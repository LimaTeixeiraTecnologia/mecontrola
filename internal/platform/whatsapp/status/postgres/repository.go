package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

type messageStatusRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewMessageStatusRepository(o11y observability.Observability, db database.DBTX) status.MessageStatusRepository {
	return &messageStatusRepository{o11y: o11y, db: db}
}

func (r *messageStatusRepository) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *messageStatusRepository) Record(ctx context.Context, record status.StatusRecord) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "whatsapp.status.repository.record")
	defer span.End()

	id, err := uuid.NewV7()
	if err != nil {
		return false, fmt.Errorf("whatsapp.status: gerar id: %w", err)
	}

	const q = `
		INSERT INTO mecontrola.whatsapp_message_status
			(id, message_id, status, recipient_id, error_code, error_title, status_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (message_id, status) DO NOTHING`

	result, err := r.conn(ctx).ExecContext(
		ctx,
		q,
		id,
		record.MessageID,
		record.Status,
		record.RecipientID,
		nullable(record.ErrorCode),
		nullable(record.ErrorTitle),
		record.StatusAt,
	)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("whatsapp.status: record: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("whatsapp.status: rows affected: %w", err)
	}
	return rows > 0, nil
}

func (r *messageStatusRepository) DeliveryCounts(ctx context.Context, messageID string) (status.DeliveryCounts, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "whatsapp.status.repository.delivery_counts")
	defer span.End()

	const q = `
		SELECT
			count(*) AS total,
			count(*) FILTER (WHERE status = $2) AS failed
		FROM mecontrola.whatsapp_message_status
		WHERE message_id = $1`

	var counts status.DeliveryCounts
	err := r.conn(ctx).QueryRowContext(ctx, q, messageID, status.DeliveryStateFailed.String()).Scan(&counts.Total, &counts.Failed)
	if err != nil {
		span.RecordError(err)
		return status.DeliveryCounts{}, fmt.Errorf("whatsapp.status: delivery_counts: %w", err)
	}
	return counts, nil
}

func nullable(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
