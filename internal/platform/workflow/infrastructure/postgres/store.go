package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const prefixWorkflowStore = "workflow.store.pg:"

type postgresStore struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewPostgresStore(o11y observability.Observability, db database.DBTX) workflow.Store {
	return &postgresStore{o11y: o11y, db: db}
}

func (s *postgresStore) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return s.db
}

func (s *postgresStore) Insert(ctx context.Context, snap workflow.Snapshot) error {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.workflow_runs
		       (id, workflow, correlation_key, status, suspend_reason, cursor,
		        state, attempts, max_attempts, version, last_error, created_at, updated_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	state := snap.State
	if len(state) == 0 {
		state = []byte("{}")
	}

	_, err := s.conn(ctx).ExecContext(ctx, query,
		snap.RunID,
		snap.Workflow,
		snap.CorrelationKey,
		snap.Status.String(),
		snap.SuspendReason.String(),
		snap.Cursor,
		state,
		snap.Attempts,
		snap.MaxAttempts,
		snap.Version,
		snap.LastError,
		snap.CreatedAt.UTC(),
		snap.UpdatedAt.UTC(),
		nullableTime(snap.EndedAt),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return workflow.ErrRunAlreadyExists
		}
		span.RecordError(err)
		return fmt.Errorf("%s insert: %w", prefixWorkflowStore, err)
	}
	return nil
}

func (s *postgresStore) Load(ctx context.Context, wf, key string) (workflow.Snapshot, bool, error) {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.load")
	defer span.End()

	const query = `
		SELECT id, workflow, correlation_key, status, suspend_reason, cursor,
		       state, attempts, max_attempts, version, last_error, created_at, updated_at, ended_at
		  FROM mecontrola.workflow_runs
		 WHERE workflow = $1
		   AND correlation_key = $2
		   AND status IN ('running','suspended')
		 LIMIT 1
	`

	row := s.conn(ctx).QueryRowContext(ctx, query, wf, key)
	snap, err := s.scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return workflow.Snapshot{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return workflow.Snapshot{}, false, fmt.Errorf("%s load: %w", prefixWorkflowStore, err)
	}
	return snap, true, nil
}

func (s *postgresStore) LoadLatest(ctx context.Context, wf, key string) (workflow.Snapshot, bool, error) {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.load_latest")
	defer span.End()

	const query = `
		SELECT id, workflow, correlation_key, status, suspend_reason, cursor,
		       state, attempts, max_attempts, version, last_error, created_at, updated_at, ended_at
		  FROM mecontrola.workflow_runs
		 WHERE workflow = $1
		   AND correlation_key = $2
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1
	`

	row := s.conn(ctx).QueryRowContext(ctx, query, wf, key)
	snap, err := s.scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return workflow.Snapshot{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return workflow.Snapshot{}, false, fmt.Errorf("%s load_latest: %w", prefixWorkflowStore, err)
	}
	return snap, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *postgresStore) scanSnapshot(row scanner) (workflow.Snapshot, error) {
	var (
		id          uuid.UUID
		wfName      string
		corrKey     string
		statusStr   string
		suspendStr  string
		cursor      int
		state       []byte
		attempts    int
		maxAttempts int
		version     int64
		lastError   string
		createdAt   time.Time
		updatedAt   time.Time
		endedAt     sql.NullTime
	)

	if err := row.Scan(
		&id, &wfName, &corrKey, &statusStr, &suspendStr, &cursor,
		&state, &attempts, &maxAttempts, &version, &lastError,
		&createdAt, &updatedAt, &endedAt,
	); err != nil {
		return workflow.Snapshot{}, err
	}

	status, err := workflow.ParseRunStatus(statusStr)
	if err != nil {
		return workflow.Snapshot{}, fmt.Errorf("%s parse status: %w", prefixWorkflowStore, err)
	}

	suspendReason, _ := workflow.ParseSuspendReason(suspendStr)

	var endedAtPtr *time.Time
	if endedAt.Valid {
		t := endedAt.Time
		endedAtPtr = &t
	}

	return workflow.Snapshot{
		RunID:          id,
		Workflow:       wfName,
		CorrelationKey: corrKey,
		Status:         status,
		SuspendReason:  suspendReason,
		Cursor:         cursor,
		State:          state,
		Attempts:       attempts,
		MaxAttempts:    maxAttempts,
		Version:        version,
		LastError:      lastError,
		CreatedAt:      createdAt.UTC(),
		UpdatedAt:      updatedAt.UTC(),
		EndedAt:        endedAtPtr,
	}, nil
}

func (s *postgresStore) Save(ctx context.Context, snap workflow.Snapshot, expectedVersion int64) error {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.save")
	defer span.End()

	const query = `
		UPDATE mecontrola.workflow_runs
		   SET status         = $1,
		       suspend_reason = $2,
		       cursor         = $3,
		       state          = $4,
		       attempts       = $5,
		       last_error     = $6,
		       version        = version + 1,
		       updated_at     = $7,
		       ended_at       = $8
		 WHERE id = $9
		   AND version = $10
	`

	state := snap.State
	if len(state) == 0 {
		state = []byte("{}")
	}

	result, err := s.conn(ctx).ExecContext(ctx, query,
		snap.Status.String(),
		snap.SuspendReason.String(),
		snap.Cursor,
		state,
		snap.Attempts,
		snap.LastError,
		snap.UpdatedAt.UTC(),
		nullableTime(snap.EndedAt),
		snap.RunID,
		expectedVersion,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s save: %w", prefixWorkflowStore, err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		span.RecordError(rowsErr)
		return fmt.Errorf("%s save rows_affected: %w", prefixWorkflowStore, rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("%s %w", prefixWorkflowStore, workflow.ErrVersionConflict)
	}
	return nil
}

func (s *postgresStore) AppendStep(ctx context.Context, rec workflow.StepRecord) error {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.append_step")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.workflow_steps
		       (id, run_id, step_id, seq, status, attempt, duration_ms, error, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (run_id, seq, attempt)
		DO UPDATE SET
		       step_id     = EXCLUDED.step_id,
		       status      = EXCLUDED.status,
		       duration_ms = EXCLUDED.duration_ms,
		       error       = EXCLUDED.error,
		       started_at  = EXCLUDED.started_at,
		       ended_at    = EXCLUDED.ended_at
	`

	_, err := s.conn(ctx).ExecContext(ctx, query,
		rec.ID,
		rec.RunID,
		rec.StepID,
		rec.Seq,
		rec.Status.String(),
		rec.Attempt,
		rec.DurationMs,
		rec.Error,
		rec.StartedAt.UTC(),
		nullableTime(rec.EndedAt),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s append_step: %w", prefixWorkflowStore, err)
	}
	return nil
}

func (s *postgresStore) ListSuspended(ctx context.Context, workflowName string, updatedBefore time.Time, limit int) ([]workflow.Snapshot, error) {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.list_suspended")
	defer span.End()

	const query = `
		SELECT id, workflow, correlation_key, status, suspend_reason, cursor,
		       state, attempts, max_attempts, version, last_error, created_at, updated_at, ended_at
		  FROM mecontrola.workflow_runs
		 WHERE workflow = $1
		   AND status = 'suspended'
		   AND updated_at < $2
		 ORDER BY updated_at ASC
		 LIMIT $3
	`

	rows, err := s.conn(ctx).QueryContext(ctx, query, workflowName, updatedBefore.UTC(), limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("%s list_suspended: %w", prefixWorkflowStore, err)
	}
	defer func() { _ = rows.Close() }()

	var result []workflow.Snapshot
	for rows.Next() {
		snap, scanErr := s.scanSnapshot(rows)
		if scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("%s list_suspended scan: %w", prefixWorkflowStore, scanErr)
		}
		result = append(result, snap)
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("%s list_suspended rows: %w", prefixWorkflowStore, err)
	}
	return result, nil
}

func (s *postgresStore) DeleteCompleted(ctx context.Context, retention time.Duration, limit int) (int64, error) {
	ctx, span := s.o11y.Tracer().Start(ctx, "workflow.store.pg.delete_completed")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.workflow_runs
		 WHERE id IN (
		     SELECT id
		       FROM mecontrola.workflow_runs
		      WHERE status IN ('succeeded','failed')
		        AND ended_at < $1
		      LIMIT $2
		 )
	`

	cutoff := time.Now().UTC().Add(-retention)
	result, err := s.conn(ctx).ExecContext(ctx, query, cutoff, limit)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("%s delete_completed: %w", prefixWorkflowStore, err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		span.RecordError(rowsErr)
		return 0, fmt.Errorf("%s delete_completed rows_affected: %w", prefixWorkflowStore, rowsErr)
	}
	return affected, nil
}
