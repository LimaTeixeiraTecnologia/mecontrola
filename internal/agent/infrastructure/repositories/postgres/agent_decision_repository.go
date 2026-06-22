package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const prefixAgentDecisionRepository = "agent.decision.repository.pg:"

type agentDecisionRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewAgentDecisionRepository(o11y observability.Observability, db database.DBTX) interfaces.AgentDecisionRepository {
	return &agentDecisionRepository{o11y: o11y, db: db}
}

func (r *agentDecisionRepository) Insert(ctx context.Context, decision entities.AgentDecision) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.decision.repository.pg.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.agent_decisions
		       (id, user_id, channel, message_id, intent_kind, prompt_sha256, llm_model,
		        redacted_response, trace_id, decided_action, resulting_event_id, status, created_at, settled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	eventID, hasEvent := decision.ResultingEventID()
	settledAt, hasSettled := decision.SettledAt()

	_, err := r.db.ExecContext(ctx, query,
		decision.ID(),
		decision.UserID(),
		decision.Channel(),
		decision.MessageID(),
		decision.IntentKind(),
		decision.PromptSHA256(),
		decision.LLMModel(),
		jsonOrDefault(decision.RedactedResponse(), "{}"),
		decision.TraceID(),
		decision.DecidedAction(),
		nullableUUID(eventID, hasEvent),
		decision.Status().String(),
		decision.CreatedAt().UTC(),
		nullableTime(settledAt, hasSettled),
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%s %w", prefixAgentDecisionRepository, interfaces.ErrAgentDecisionConflict)
		}
		return fmt.Errorf("%s insert: %w", prefixAgentDecisionRepository, err)
	}
	return nil
}

func (r *agentDecisionRepository) FindByMessage(ctx context.Context, userID uuid.UUID, channel, messageID string) (interfaces.AgentDecisionSnapshot, bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.decision.repository.pg.find_by_message")
	defer span.End()

	const query = `
		SELECT status, redacted_response
		  FROM mecontrola.agent_decisions
		 WHERE user_id = $1 AND channel = $2 AND message_id = $3
	`

	var snapshot interfaces.AgentDecisionSnapshot
	err := r.db.QueryRowContext(ctx, query, userID, channel, messageID).Scan(&snapshot.Status, &snapshot.RedactedResponse)
	if errors.Is(err, sql.ErrNoRows) {
		return interfaces.AgentDecisionSnapshot{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return interfaces.AgentDecisionSnapshot{}, false, fmt.Errorf("%s find_by_message: %w", prefixAgentDecisionRepository, err)
	}
	return snapshot, true, nil
}

func (r *agentDecisionRepository) UpdateSettlement(ctx context.Context, decision entities.AgentDecision) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.decision.repository.pg.update_settlement")
	defer span.End()

	const query = `
		UPDATE mecontrola.agent_decisions
		   SET status             = $1,
		       resulting_event_id = $2,
		       redacted_response  = $3,
		       settled_at         = $4
		 WHERE id = $5
	`

	eventID, hasEvent := decision.ResultingEventID()
	settledAt, hasSettled := decision.SettledAt()

	result, err := r.db.ExecContext(ctx, query,
		decision.Status().String(),
		nullableUUID(eventID, hasEvent),
		jsonOrDefault(decision.RedactedResponse(), "{}"),
		nullableTime(settledAt, hasSettled),
		decision.ID(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s update_settlement: %w", prefixAgentDecisionRepository, err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		span.RecordError(rowsErr)
		return fmt.Errorf("%s update_settlement rows_affected: %w", prefixAgentDecisionRepository, rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("%s %w", prefixAgentDecisionRepository, interfaces.ErrAgentDecisionNotFound)
	}
	return nil
}
