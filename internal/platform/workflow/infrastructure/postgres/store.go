package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

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

	err := s.conn(ctx).QueryRowContext(ctx, query, wf, key).Scan(
		&id, &wfName, &corrKey, &statusStr, &suspendStr, &cursor,
		&state, &attempts, &maxAttempts, &version, &lastError,
		&createdAt, &updatedAt, &endedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return workflow.Snapshot{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return workflow.Snapshot{}, false, fmt.Errorf("%s load: %w", prefixWorkflowStore, err)
	}

	status, parseErr := workflow.ParseRunStatus(statusStr)
	if parseErr != nil {
		return workflow.Snapshot{}, false, fmt.Errorf("%s load parse status: %w", prefixWorkflowStore, parseErr)
	}

	suspendReason, _ := workflow.ParseSuspendReason(suspendStr)

	var endedAtPtr *time.Time
	if endedAt.Valid {
		t := endedAt.Time
		endedAtPtr = &t
	}

	snap := workflow.Snapshot{
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
	}
	return snap, true, nil
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
