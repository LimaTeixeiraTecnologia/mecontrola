package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

// PgxWebhookEventRepository implementa WebhookEventRepository usando pgx/v5.
// InsertIfNew e RecordApplication usam ON CONFLICT DO NOTHING para garantir idempotência (ADR-009).
type PgxWebhookEventRepository struct {
	mgr    *database.Manager
	mapper rowMapper
}

func NewPgxWebhookEventRepository(mgr *database.Manager) *PgxWebhookEventRepository {
	return &PgxWebhookEventRepository{mgr: mgr}
}

func (r *PgxWebhookEventRepository) dbtx(ctx context.Context) database.DBTX {
	return r.mgr.DBTX(ctx)
}

// InsertIfNew persiste o evento via INSERT ... ON CONFLICT DO NOTHING.
// Retorna (false, nil) em duplicata — não é erro.
func (r *PgxWebhookEventRepository) InsertIfNew(ctx context.Context, event entities.WebhookEvent) (bool, error) {
	result, err := r.dbtx(ctx).ExecContext(ctx, insertIfNewWebhookEvent,
		event.ID().String(),
		event.Provider(),
		event.ExternalEventID().String(),
		event.EventType(),
		event.Signature(),
		[]byte(event.HeadersJSON()),
		[]byte(event.Payload()),
		event.ReceivedAt(),
	)
	if err != nil {
		return false, fmt.Errorf("postgres webhook event repository: insert if new: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("postgres webhook event repository: rows affected: %w", err)
	}
	return affected > 0, nil
}

func (r *PgxWebhookEventRepository) FindRawPayload(ctx context.Context, id valueobjects.WebhookEventID) (json.RawMessage, error) {
	var payload []byte
	err := r.dbtx(ctx).QueryRowContext(ctx, findRawPayload, id.String()).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWebhookEventNotFound
		}
		return nil, fmt.Errorf("postgres webhook event repository: find raw payload: %w", err)
	}
	return json.RawMessage(payload), nil
}

func (r *PgxWebhookEventRepository) MarkProcessed(ctx context.Context, id valueobjects.WebhookEventID, at time.Time) error {
	_, err := r.dbtx(ctx).ExecContext(ctx, markProcessed, id.String(), at)
	if err != nil {
		return fmt.Errorf("postgres webhook event repository: mark processed: %w", err)
	}
	return nil
}

// RecordApplication registra a aplicação do evento em billing_event_applications via ON CONFLICT DO NOTHING.
// Retorna (false, nil) em conflict — idempotência do processor (ADR-009).
func (r *PgxWebhookEventRepository) RecordApplication(ctx context.Context, eventID valueobjects.WebhookEventID, subID entities.SubscriptionID, at time.Time) (bool, error) {
	result, err := r.dbtx(ctx).ExecContext(ctx, recordApplication,
		eventID.String(),
		subID.String(),
		at,
	)
	if err != nil {
		return false, fmt.Errorf("postgres webhook event repository: record application: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("postgres webhook event repository: rows affected: %w", err)
	}
	return affected > 0, nil
}

func (r *PgxWebhookEventRepository) ListPendingAnonymization(ctx context.Context, olderThan time.Time, limit int) ([]entities.WebhookEvent, error) {
	rows, err := r.dbtx(ctx).QueryContext(ctx, listPendingAnonymization, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres webhook event repository: list pending anonymization: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]entities.WebhookEvent, 0, limit)
	for rows.Next() {
		var wr webhookEventRow
		scanErr := rows.Scan(
			&wr.ID,
			&wr.Provider,
			&wr.ExternalEventID,
			&wr.EventType,
			&wr.Signature,
			&wr.HeadersJSON,
			&wr.Payload,
			&wr.ReceivedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres webhook event repository: scan row: %w", scanErr)
		}
		event, hydrateErr := r.mapper.hydrateWebhookEvent(wr)
		if hydrateErr != nil {
			return nil, fmt.Errorf("postgres webhook event repository: hydrate: %w", hydrateErr)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres webhook event repository: rows error: %w", err)
	}
	return result, nil
}

func (r *PgxWebhookEventRepository) Anonymize(ctx context.Context, id valueobjects.WebhookEventID, redacted json.RawMessage, at time.Time) error {
	_, err := r.dbtx(ctx).ExecContext(ctx, anonymize, id.String(), []byte(redacted), at)
	if err != nil {
		return fmt.Errorf("postgres webhook event repository: anonymize: %w", err)
	}
	return nil
}
