package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type kiwifyEventRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewKiwifyEventRepository(o11y observability.Observability, db database.DBTX) interfaces.KiwifyEventRepository {
	return &kiwifyEventRepository{o11y: o11y, db: db}
}

func (r *kiwifyEventRepository) Persist(ctx context.Context, envelopeID string, trigger string, rawBody []byte, signatureStatus string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.kiwify_event.persist")
	defer span.End()

	const query = `
		INSERT INTO billing_kiwify_events (envelope_id, trigger, raw_body, signature_status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (envelope_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, envelopeID, trigger, rawBody, signatureStatus)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("billing/postgres: persist_kiwify_event: %w", err)
	}
	return nil
}

func (r *kiwifyEventRepository) MarkProcessed(ctx context.Context, envelopeID string, processedAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.kiwify_event.mark_processed")
	defer span.End()

	const query = `
		UPDATE billing_kiwify_events
		   SET processed_at = $1
		 WHERE envelope_id = $2
	`

	_, err := r.db.ExecContext(ctx, query, processedAt, envelopeID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("billing/postgres: mark_processed_kiwify_event: %w", err)
	}
	return nil
}

func (r *kiwifyEventRepository) DeleteOlderThan(ctx context.Context, before time.Time, limit int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.kiwify_event.delete_older_than")
	defer span.End()

	query := fmt.Sprintf(`
		DELETE FROM billing_kiwify_events
		 WHERE envelope_id IN (
		       SELECT envelope_id
		         FROM billing_kiwify_events
		        WHERE received_at < $1
		        LIMIT %d
		 )
	`, limit)

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("billing/postgres: delete_older_than_kiwify_event: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("billing/postgres: delete_older_than_kiwify_event rows_affected: %w", err)
	}
	return n, nil
}
