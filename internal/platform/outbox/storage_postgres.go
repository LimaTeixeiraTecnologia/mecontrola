package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
)

type postgresStorage struct {
	db manager.Manager
}

func NewPostgresStorage(db manager.Manager) Storage {
	return &postgresStorage{db: db}
}

func (s *postgresStorage) Insert(ctx context.Context, evt Event, maxAttempts int) error {
	dbtx, ok := database.FromContext(ctx)
	if !ok {
		return ErrNoActiveTransaction
	}
	meta, err := marshalMetadata(evt.Metadata)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO outbox_events
			(id, event_type, aggregate_type, aggregate_id, payload, metadata,
			 status, attempts, max_attempts, next_attempt_at, occurred_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,$8,$9,$10,now(),now())
		ON CONFLICT (id) DO NOTHING`

	_, err = dbtx.ExecContext(ctx, q,
		evt.ID,
		evt.Type,
		evt.AggregateType,
		evt.AggregateID,
		evt.Payload,
		meta,
		int(StatusPending),
		maxAttempts,
		evt.OccurredAt,
		evt.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("outbox: insert event: %w", err)
	}
	return nil
}

func (s *postgresStorage) ClaimBatch(ctx context.Context, lockedBy string, batchSize int) ([]Row, error) {
	tx, err := s.db.BeginTx(ctx, database.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("outbox: claim batch begin tx: %w", err)
	}

	const selectQ = `
		SELECT id, event_type, aggregate_type, aggregate_id, payload, metadata,
		       attempts, max_attempts, occurred_at
		  FROM outbox_events
		 WHERE status = $1
		   AND next_attempt_at <= now()
		 ORDER BY next_attempt_at
		 LIMIT $2
		   FOR UPDATE SKIP LOCKED`

	rows, err := tx.QueryContext(ctx, selectQ, int(StatusPending), batchSize)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("outbox: claim batch select: %w", err), tx.Rollback(ctx))
	}

	var result []Row
	for rows.Next() {
		var r Row
		var meta []byte
		if err := rows.Scan(
			&r.ID,
			&r.Type,
			&r.AggregateType,
			&r.AggregateID,
			&r.Payload,
			&meta,
			&r.Attempts,
			&r.MaxAttempts,
			&r.OccurredAt,
		); err != nil {
			return nil, errors.Join(fmt.Errorf("outbox: claim batch scan: %w", err), rows.Close(), tx.Rollback(ctx))
		}
		m, err := unmarshalMetadata(meta)
		if err != nil {
			return nil, errors.Join(err, rows.Close(), tx.Rollback(ctx))
		}
		r.Metadata = m
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("outbox: claim batch rows: %w", err), rows.Close(), tx.Rollback(ctx))
	}
	if err := rows.Close(); err != nil {
		return nil, errors.Join(fmt.Errorf("outbox: claim batch close rows: %w", err), tx.Rollback(ctx))
	}

	if len(result) == 0 {
		return nil, tx.Rollback(ctx)
	}

	const updateQ = `
		UPDATE outbox_events
		   SET status = $1, locked_at = now(), locked_by = $2, updated_at = now()
		 WHERE id = $3`

	for _, r := range result {
		if _, err := tx.ExecContext(ctx, updateQ, int(StatusProcessing), lockedBy, r.ID); err != nil {
			return nil, errors.Join(fmt.Errorf("outbox: claim batch update %s: %w", r.ID, err), tx.Rollback(ctx))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("outbox: claim batch commit: %w", err), tx.Rollback(ctx))
	}
	return result, nil
}

func (s *postgresStorage) MarkPublished(ctx context.Context, id string) error {
	const q = `
		UPDATE outbox_events
		   SET status = $1, published_at = now(), locked_at = NULL, locked_by = NULL, updated_at = now()
		 WHERE id = $2`

	if _, err := s.db.DBTX(ctx).ExecContext(ctx, q, int(StatusPublished), id); err != nil {
		return fmt.Errorf("outbox: mark published: %w", err)
	}
	return nil
}

func (s *postgresStorage) MarkPendingRetry(ctx context.Context, id string, lastErr string, nextAttemptAt time.Time) error {
	const q = `
		UPDATE outbox_events
		   SET status = $1, attempts = attempts + 1, last_error = $2,
		       next_attempt_at = $3, locked_at = NULL, locked_by = NULL, updated_at = now()
		 WHERE id = $4`

	if _, err := s.db.DBTX(ctx).ExecContext(ctx, q, int(StatusPending), lastErr, nextAttemptAt, id); err != nil {
		return fmt.Errorf("outbox: mark pending retry: %w", err)
	}
	return nil
}

func (s *postgresStorage) MarkFailed(ctx context.Context, id string, lastErr string) error {
	const q = `
		UPDATE outbox_events
		   SET status = $1, attempts = attempts + 1, last_error = $2,
		       locked_at = NULL, locked_by = NULL, updated_at = now()
		 WHERE id = $3`

	if _, err := s.db.DBTX(ctx).ExecContext(ctx, q, int(StatusFailed), lastErr, id); err != nil {
		return fmt.Errorf("outbox: mark failed: %w", err)
	}
	return nil
}

func (s *postgresStorage) ResetStuck(ctx context.Context, stuckAfter time.Duration) (int64, error) {
	const q = `
		UPDATE outbox_events
		   SET status = $1, locked_at = NULL, locked_by = NULL, updated_at = now()
		 WHERE status = $2
		   AND locked_at < now() - ($3 * interval '1 microsecond')`

	res, err := s.db.DBTX(ctx).ExecContext(ctx, q, int(StatusPending), int(StatusProcessing), stuckAfter.Microseconds())
	if err != nil {
		return 0, fmt.Errorf("outbox: reset stuck: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: reset stuck rows affected: %w", err)
	}
	return n, nil
}

func (s *postgresStorage) DeletePublishedBatch(ctx context.Context, retention time.Duration, limit int) (int64, error) {
	const q = `
		DELETE FROM outbox_events
		 WHERE id IN (
		   SELECT id FROM outbox_events
		    WHERE status = $1
		      AND published_at < now() - ($2 * interval '1 microsecond')
		    ORDER BY published_at
		    LIMIT $3
		 )`

	res, err := s.db.DBTX(ctx).ExecContext(ctx, q, int(StatusPublished), retention.Microseconds(), limit)
	if err != nil {
		return 0, fmt.Errorf("outbox: delete published batch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: delete published batch rows affected: %w", err)
	}
	return n, nil
}

func marshalMetadata(m map[string]string) ([]byte, error) {
	if len(m) == 0 {
		return []byte(`{}`), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("outbox: marshal metadata: %w", err)
	}
	return b, nil
}

func unmarshalMetadata(b []byte) (map[string]string, error) {
	m := make(map[string]string)
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("outbox: unmarshal metadata: %w", err)
	}
	return m, nil
}
