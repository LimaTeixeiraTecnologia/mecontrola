package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type runStore struct {
	db database.DBTX
}

func NewRunStore(db database.DBTX) agent.RunStore {
	return &runStore{db: db}
}

func (s *runStore) Insert(ctx context.Context, run agent.Run) error {
	const q = `
		INSERT INTO mecontrola.platform_runs
			(id, platform_thread_id, resource_id, thread_id, agent_id, workflow, correlation_key, status, outcome, error, started_at, ended_at, duration_ms)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	_, err := s.db.ExecContext(ctx, q,
		run.ID, run.PlatformThreadID, run.ResourceID, run.ThreadID,
		run.AgentID, run.Workflow, run.CorrelationKey,
		run.Status.String(), outcomeText(run.Outcome), run.Error,
		run.StartedAt, run.EndedAt, run.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("agent.postgres.run_store.insert: %w", err)
	}
	return nil
}

func (s *runStore) Update(ctx context.Context, run agent.Run) error {
	const q = `
		UPDATE mecontrola.platform_runs
		SET status=$1, outcome=$2, error=$3, ended_at=$4, duration_ms=$5, agent_id=$6, workflow=$7
		WHERE id=$8`
	_, err := s.db.ExecContext(ctx, q,
		run.Status.String(), outcomeText(run.Outcome), run.Error,
		run.EndedAt, run.DurationMs,
		run.AgentID, run.Workflow,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("agent.postgres.run_store.update: %w", err)
	}
	return nil
}

func (s *runStore) Load(ctx context.Context, id uuid.UUID) (agent.Run, error) {
	const q = `
		SELECT id, platform_thread_id, resource_id, thread_id, agent_id, workflow, correlation_key, status, outcome, error, started_at, ended_at, duration_ms
		FROM mecontrola.platform_runs
		WHERE id=$1`
	row := s.db.QueryRowContext(ctx, q, id)
	var r agent.Run
	var statusStr string
	var outcomeStr string
	var endedAt sql.NullTime
	if err := row.Scan(
		&r.ID, &r.PlatformThreadID, &r.ResourceID, &r.ThreadID,
		&r.AgentID, &r.Workflow, &r.CorrelationKey,
		&statusStr, &outcomeStr, &r.Error,
		&r.StartedAt, &endedAt, &r.DurationMs,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agent.Run{}, agent.ErrRunNotFound
		}
		return agent.Run{}, fmt.Errorf("agent.postgres.run_store.load: %w", err)
	}
	status, _ := agent.ParseRunStatus(statusStr)
	r.Status = status
	if outcome, parseErr := agent.ParseToolOutcome(outcomeStr); parseErr == nil {
		r.Outcome = outcome
	}
	if endedAt.Valid {
		t := endedAt.Time
		r.EndedAt = &t
	}
	return r, nil
}

func outcomeText(o agent.ToolOutcome) string {
	if !o.IsValid() {
		return ""
	}
	return o.String()
}
