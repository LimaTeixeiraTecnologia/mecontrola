package postgres

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const prefixAgentRunRepository = "agent.run.repository.pg:"

type agentRunRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewAgentRunRepository(o11y observability.Observability, db database.DBTX) interfaces.AgentRunRepository {
	return &agentRunRepository{o11y: o11y, db: db}
}

func (r *agentRunRepository) Insert(ctx context.Context, run entities.Run) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.run.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_runs
		       (id, thread_id, user_id, channel, message_id, agent_id, workflow, tool_name,
		        intent_kind, schema_version, outcome, status, error, decision_id, started_at, ended_at, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	decisionID, hasDecision := run.DecisionID()
	endedAt, hasEnded := run.EndedAt()

	_, err := r.db.ExecContext(ctx, query,
		run.ID(),
		run.ThreadID(),
		run.UserID(),
		run.Channel(),
		run.MessageID(),
		run.AgentID(),
		run.Workflow(),
		run.ToolName(),
		run.IntentKind(),
		run.SchemaVersion(),
		run.Outcome(),
		run.Status().String(),
		run.ErrText(),
		nullableUUID(decisionID, hasDecision),
		run.StartedAt().UTC(),
		nullableTime(endedAt, hasEnded),
		run.DurationMs(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s insert: %w", prefixAgentRunRepository, err)
	}
	return nil
}

func (r *agentRunRepository) UpdateOnFinish(ctx context.Context, run entities.Run) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.repository.run.update_on_finish")
	defer span.End()

	const query = `
		UPDATE mecontrola.agent_runs
		   SET status      = $1,
		       outcome     = $2,
		       error       = $3,
		       workflow    = $4,
		       tool_name   = $5,
		       intent_kind = $6,
		       ended_at    = $7,
		       duration_ms = $8
		 WHERE id = $9
	`

	endedAt, hasEnded := run.EndedAt()

	result, err := r.db.ExecContext(ctx, query,
		run.Status().String(),
		run.Outcome(),
		run.ErrText(),
		run.Workflow(),
		run.ToolName(),
		run.IntentKind(),
		nullableTime(endedAt, hasEnded),
		run.DurationMs(),
		run.ID(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s update_on_finish: %w", prefixAgentRunRepository, err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		span.RecordError(rowsErr)
		return fmt.Errorf("%s update_on_finish rows_affected: %w", prefixAgentRunRepository, rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("%s %w", prefixAgentRunRepository, interfaces.ErrAgentRunNotFound)
	}
	return nil
}
