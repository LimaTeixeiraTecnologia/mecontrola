package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
)

type messageRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewMessageRepository(o11y observability.Observability, db database.DBTX) dedup.MessageRepository {
	return &messageRepository{o11y: o11y, db: db}
}

func (r *messageRepository) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

const channelWhatsApp = "whatsapp"

func (r *messageRepository) InsertIfAbsent(ctx context.Context, wamid string) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "whatsapp.dedup.repository.insert_if_absent")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ($1, $2, now())
		ON CONFLICT (channel, message_id) DO NOTHING`

	result, err := r.conn(ctx).ExecContext(ctx, q, channelWhatsApp, wamid)
	if err != nil {
		return false, fmt.Errorf("whatsapp.dedup: insert_if_absent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("whatsapp.dedup: rows affected: %w", err)
	}
	return rows > 0, nil
}

func (r *messageRepository) DeleteProcessedBefore(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "whatsapp.dedup.repository.delete_processed_before")
	defer span.End()

	const q = `
		DELETE FROM mecontrola.channel_processed_messages
		 WHERE (channel, message_id) IN (
			SELECT channel, message_id
			  FROM mecontrola.channel_processed_messages
			 WHERE channel = $1 AND processed_at <= $2
			 ORDER BY processed_at
			 LIMIT $3
		)`

	result, err := r.conn(ctx).ExecContext(ctx, q, channelWhatsApp, before, batchSize)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("whatsapp.dedup: delete_processed_before: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("whatsapp.dedup: delete_processed_before rows affected: %w", err)
	}
	return rows, nil
}
