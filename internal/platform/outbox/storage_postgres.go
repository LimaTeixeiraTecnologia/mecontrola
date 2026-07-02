package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type postgresStorage struct {
	db                 database.DBTX
	claimDeferredTotal observability.Counter
}

func NewPostgresStorage(db database.DBTX) OutboxRepository {
	return &postgresStorage{db: db}
}

func NewObservablePostgresStorage(db database.DBTX, claimDeferredTotal observability.Counter) OutboxRepository {
	return &postgresStorage{db: db, claimDeferredTotal: claimDeferredTotal}
}

func (s *postgresStorage) Insert(ctx context.Context, evt Event, maxAttempts int) error {
	meta, err := marshalMetadata(evt.Metadata)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO outbox_events
			(id, event_type, aggregate_type, aggregate_id, aggregate_user_id, payload, metadata,
			 status, attempts, max_attempts, next_attempt_at, occurred_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,$9,now(),$10,now(),now())
		ON CONFLICT (id) DO NOTHING`

	_, err = s.db.ExecContext(ctx, q,
		evt.ID,
		evt.Type,
		evt.AggregateType,
		evt.AggregateID,
		nilIfEmpty(evt.AggregateUserID),
		evt.Payload,
		meta,
		int(StatusPending),
		maxAttempts,
		evt.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("outbox: insert event: %w", err)
	}
	return nil
}

func (s *postgresStorage) ClaimBatch(ctx context.Context, lockedBy string, batchSize int) ([]Row, error) {
	const claimQ = `
		WITH claimable AS (
		  SELECT id
		    FROM mecontrola.outbox_events o
		   WHERE o.status = 1
		     AND o.next_attempt_at <= now()
		     AND (
		          o.aggregate_user_id IS NULL
		       OR NOT EXISTS (
		            SELECT 1 FROM mecontrola.outbox_events p
		             WHERE p.aggregate_user_id = o.aggregate_user_id
		               AND p.status = 2)
		     )
		     AND NOT EXISTS (
		            SELECT 1 FROM mecontrola.outbox_events e2
		             WHERE e2.aggregate_user_id = o.aggregate_user_id
		               AND e2.status = 1
		               AND (e2.occurred_at, e2.created_at, e2.id) < (o.occurred_at, o.created_at, o.id))
		   ORDER BY o.occurred_at, o.created_at, o.id
		   LIMIT $2
		   FOR UPDATE SKIP LOCKED
		)
		UPDATE mecontrola.outbox_events t
		   SET status = 2, locked_at = now(), locked_by = $1, updated_at = now()
		  FROM claimable c
		 WHERE t.id = c.id
		RETURNING t.id, t.event_type, t.aggregate_type, t.aggregate_id, t.aggregate_user_id,
		          t.payload, t.metadata, t.attempts, t.max_attempts, t.occurred_at`

	rows, err := s.db.QueryContext(ctx, claimQ, lockedBy, batchSize)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			slog.WarnContext(ctx, "outbox: claim batch deferred", slog.String("reason", "unique_violation"))
			if s.claimDeferredTotal != nil {
				s.claimDeferredTotal.Add(ctx, 1)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("outbox: claim batch: %w", err)
	}

	var result []Row
	for rows.Next() {
		var r Row
		var meta []byte
		var aggregateUserID sql.NullString
		if err := rows.Scan(
			&r.ID,
			&r.Type,
			&r.AggregateType,
			&r.AggregateID,
			&aggregateUserID,
			&r.Payload,
			&meta,
			&r.Attempts,
			&r.MaxAttempts,
			&r.OccurredAt,
		); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("outbox: claim batch scan: %w", err)
		}
		if aggregateUserID.Valid {
			r.AggregateUserID = aggregateUserID.String
		}
		m, err := unmarshalMetadata(meta)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.Metadata = m
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("outbox: claim batch rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("outbox: claim batch close: %w", err)
	}

	return result, nil
}

func (s *postgresStorage) MarkPublished(ctx context.Context, id string) error {
	const q = `
		UPDATE outbox_events
		   SET status = $1, published_at = now(), locked_at = NULL, locked_by = NULL, updated_at = now()
		 WHERE id = $2`

	if _, err := s.db.ExecContext(ctx, q, int(StatusPublished), id); err != nil {
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

	if _, err := s.db.ExecContext(ctx, q, int(StatusPending), lastErr, nextAttemptAt, id); err != nil {
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

	if _, err := s.db.ExecContext(ctx, q, int(StatusFailed), lastErr, id); err != nil {
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

	res, err := s.db.ExecContext(ctx, q, int(StatusPending), int(StatusProcessing), stuckAfter.Microseconds())
	if err != nil {
		return 0, fmt.Errorf("outbox: reset stuck: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: reset stuck rows affected: %w", err)
	}
	return n, nil
}

func (s *postgresStorage) CountPending(ctx context.Context) (int64, error) {
	const q = `
		SELECT COUNT(*)
		  FROM outbox_events
		 WHERE status = $1
		   AND next_attempt_at <= now()`

	var count int64
	if err := s.db.QueryRowContext(ctx, q, int(StatusPending)).Scan(&count); err != nil {
		return 0, fmt.Errorf("outbox: count pending: %w", err)
	}
	return count, nil
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

	res, err := s.db.ExecContext(ctx, q, int(StatusPublished), retention.Microseconds(), limit)
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

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
