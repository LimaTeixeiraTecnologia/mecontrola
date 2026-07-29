package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type ConsumerDedupRepository struct {
	db database.DBTX
}

func NewConsumerDedupRepository(db database.DBTX) *ConsumerDedupRepository {
	return &ConsumerDedupRepository{db: db}
}

func (r *ConsumerDedupRepository) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *ConsumerDedupRepository) InsertIfAbsent(ctx context.Context, consumer, messageID string) (bool, error) {
	const q = `
		INSERT INTO mecontrola.consumer_processed_messages (consumer, message_id, processed_at)
		VALUES ($1, $2, now())
		ON CONFLICT (consumer, message_id) DO NOTHING`

	result, err := r.conn(ctx).ExecContext(ctx, q, consumer, messageID)
	if err != nil {
		return false, fmt.Errorf("whatsapp.consumer_dedup: insert_if_absent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("whatsapp.consumer_dedup: rows affected: %w", err)
	}
	return rows > 0, nil
}

func (r *ConsumerDedupRepository) Delete(ctx context.Context, consumer, messageID string) error {
	const q = `
		DELETE FROM mecontrola.consumer_processed_messages
		 WHERE consumer = $1 AND message_id = $2`

	if _, err := r.conn(ctx).ExecContext(ctx, q, consumer, messageID); err != nil {
		return fmt.Errorf("whatsapp.consumer_dedup: delete: %w", err)
	}
	return nil
}

func (r *ConsumerDedupRepository) DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	const q = `
		DELETE FROM mecontrola.consumer_processed_messages
		 WHERE processed_at <= $1`

	result, err := r.conn(ctx).ExecContext(ctx, q, cutoff)
	if err != nil {
		return 0, fmt.Errorf("whatsapp.consumer_dedup: delete_processed_before: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("whatsapp.consumer_dedup: delete_processed_before rows affected: %w", err)
	}
	return rows, nil
}
