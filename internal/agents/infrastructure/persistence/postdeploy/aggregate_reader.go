package postdeploy

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/postdeploy"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type aggregateReader struct {
	db database.DBTX
}

func NewAggregateReader(db database.DBTX) postdeploy.AggregateReader {
	return &aggregateReader{db: db}
}

func (r *aggregateReader) RunAggregate(ctx context.Context, agentID string, since time.Time) (postdeploy.RunAggregate, error) {
	const q = `
		SELECT
			count(*) AS total_runs,
			count(*) FILTER (WHERE status = $3) AS succeeded_runs,
			count(*) FILTER (WHERE status = $4) AS failed_runs,
			count(*) FILTER (WHERE outcome NOT IN ($5, $6) OR outcome = '') AS expected_tool_runs,
			count(*) FILTER (WHERE outcome = $7) AS truncated_runs,
			min(started_at) AS window_start,
			max(started_at) AS window_end
		FROM mecontrola.platform_runs
		WHERE agent_id = $1 AND started_at >= $2`

	row := r.db.QueryRowContext(ctx, q,
		agentID, since,
		agent.RunStatusSucceeded.String(),
		agent.RunStatusFailed.String(),
		agent.ToolOutcomeClarify.String(),
		agent.ToolOutcomeReplay.String(),
		agent.ToolOutcomeTruncated.String(),
	)

	var out postdeploy.RunAggregate
	var windowStart, windowEnd sql.NullTime
	if err := row.Scan(
		&out.TotalRuns, &out.SucceededRuns, &out.FailedRuns,
		&out.ExpectedToolRuns, &out.TruncatedRuns,
		&windowStart, &windowEnd,
	); err != nil {
		return postdeploy.RunAggregate{}, fmt.Errorf("postdeploy.postgres.aggregate_reader.run_aggregate: %w", err)
	}

	out.AgentID = agentID
	if windowStart.Valid {
		out.WindowStart = windowStart.Time
	}
	if windowEnd.Valid {
		out.WindowEnd = windowEnd.Time
	}
	return out, nil
}

func (r *aggregateReader) ExpectedToolHits(ctx context.Context, agentID string, since time.Time) (int, error) {
	const q = `
		SELECT count(*)
		FROM mecontrola.platform_runs
		WHERE agent_id = $1
		  AND started_at >= $2
		  AND status = $3
		  AND (outcome NOT IN ($4, $5) OR outcome = '')`

	var hits int
	err := r.db.QueryRowContext(ctx, q,
		agentID, since,
		agent.RunStatusSucceeded.String(),
		agent.ToolOutcomeClarify.String(),
		agent.ToolOutcomeReplay.String(),
	).Scan(&hits)
	if err != nil {
		return 0, fmt.Errorf("postdeploy.postgres.aggregate_reader.expected_tool_hits: %w", err)
	}
	return hits, nil
}

func (r *aggregateReader) ScorerAggregates(ctx context.Context, agentID string, since time.Time) (map[string]postdeploy.ScorerAggregate, error) {
	const q = `
		SELECT sr.scorer_id, count(*) AS sample_n, avg(sr.score) AS mean_score
		FROM mecontrola.platform_scorer_results sr
		JOIN mecontrola.platform_runs pr ON pr.id = sr.run_id
		WHERE pr.agent_id = $1 AND pr.started_at >= $2
		GROUP BY sr.scorer_id`

	rows, err := r.db.QueryContext(ctx, q, agentID, since)
	if err != nil {
		return nil, fmt.Errorf("postdeploy.postgres.aggregate_reader.scorer_aggregates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]postdeploy.ScorerAggregate)
	for rows.Next() {
		var agg postdeploy.ScorerAggregate
		if err := rows.Scan(&agg.ScorerID, &agg.SampleN, &agg.MeanScore); err != nil {
			return nil, fmt.Errorf("postdeploy.postgres.aggregate_reader.scorer_aggregates: scan: %w", err)
		}
		agg.AgentID = agentID
		out[agg.ScorerID] = agg
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postdeploy.postgres.aggregate_reader.scorer_aggregates: rows: %w", err)
	}
	return out, nil
}

func (r *aggregateReader) DuplicateWriteViolations(ctx context.Context, agentID string, since time.Time) (int64, error) {
	const q = `
		SELECT count(*)
		FROM mecontrola.platform_scorer_results sr
		JOIN mecontrola.platform_runs pr ON pr.id = sr.run_id
		WHERE pr.agent_id = $1
		  AND pr.started_at >= $2
		  AND sr.scorer_id = $3
		  AND sr.score < 1`

	var violations int64
	err := r.db.QueryRowContext(ctx, q, agentID, since, postdeploy.ScorerIDNoDuplicateWrite).Scan(&violations)
	if err != nil {
		return 0, fmt.Errorf("postdeploy.postgres.aggregate_reader.duplicate_write_violations: %w", err)
	}
	return violations, nil
}
