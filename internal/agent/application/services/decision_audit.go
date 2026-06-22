package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type DecisionAuditDeps struct {
	Factory interfaces.AgentDecisionRepositoryFactory
	UoW     uow.UnitOfWork
}

type decisionAuditor struct {
	factory  interfaces.AgentDecisionRepositoryFactory
	uow      uow.UnitOfWork
	redactor DecisionRedactor
	o11y     observability.Observability
}

func newDecisionAuditor(o11y observability.Observability, deps DecisionAuditDeps, redactor DecisionRedactor) *decisionAuditor {
	return &decisionAuditor{
		factory:  deps.Factory,
		uow:      deps.UoW,
		redactor: redactor,
		o11y:     o11y,
	}
}

func (a *decisionAuditor) enabled() bool {
	return a != nil && a.factory != nil && a.uow != nil
}

func (a *decisionAuditor) lookup(ctx context.Context, userID uuid.UUID, channel, messageID string) (string, bool) {
	if !a.enabled() {
		return "", false
	}
	var (
		snapshot interfaces.AgentDecisionSnapshot
		found    bool
	)
	err := a.uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		result, ok, findErr := a.factory.AgentDecisionRepository(db).FindByMessage(ctx, userID, channel, messageID)
		if findErr != nil {
			return findErr
		}
		snapshot = result
		found = ok
		return nil
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.decision_audit.lookup_failed",
			observability.String("message_id", messageID),
			observability.Error(err),
		)
		return "", false
	}
	if !found {
		return "", false
	}
	return decodeRedactedReply(snapshot.RedactedResponse), true
}

func decodeRedactedReply(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		Redacted string `json:"redacted"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return payload.Redacted
}

type decisionContext struct {
	auditor    *decisionAuditor
	pending    entities.AgentDecision
	recorded   bool
	conflicted bool
}

func (a *decisionAuditor) begin(ctx context.Context, in decisionRecordInput) decisionContext {
	if !a.enabled() {
		return decisionContext{}
	}
	if in.LLMModel == "" || in.PromptSHA256 == "" || in.MessageID == "" {
		a.o11y.Logger().Warn(ctx, "agent.decision_audit.begin_skipped",
			observability.String("message_id", in.MessageID),
			observability.String("llm_model", in.LLMModel),
			observability.String("reason", "incomplete_parsed_intent"),
		)
		return decisionContext{}
	}

	params := entities.AgentDecisionParams{
		ID:               uuid.New(),
		UserID:           in.UserID,
		Channel:          in.Channel,
		MessageID:        in.MessageID,
		IntentKind:       in.IntentKind,
		PromptSHA256:     in.PromptSHA256,
		LLMModel:         in.LLMModel,
		RedactedResponse: a.redactResponse(ctx, in.DirectReply, in.RawResponse),
		TraceID:          in.TraceID,
		DecidedAction:    in.IntentKind,
		CreatedAt:        time.Now().UTC(),
	}
	decision, err := entities.NewPendingDecision(params)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.decision_audit.build_failed", observability.Error(err))
		return decisionContext{}
	}

	insertErr := a.uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		return a.factory.AgentDecisionRepository(db).Insert(ctx, decision)
	})
	if insertErr != nil {
		if errors.Is(insertErr, interfaces.ErrAgentDecisionConflict) {
			return decisionContext{conflicted: true}
		}
		a.o11y.Logger().Warn(ctx, "agent.decision_audit.insert_failed",
			observability.String("message_id", in.MessageID),
			observability.Error(insertErr),
		)
		return decisionContext{}
	}
	return decisionContext{auditor: a, pending: decision, recorded: true}
}

func (c decisionContext) settle(ctx context.Context, executed bool) {
	if !c.recorded || c.auditor == nil {
		return
	}
	c.auditor.settle(ctx, c.pending, executed)
}

func (a *decisionAuditor) settle(ctx context.Context, pending entities.AgentDecision, executed bool) {
	var (
		settled entities.AgentDecision
		err     error
	)
	if executed {
		settled, err = pending.Execute(uuid.New(), time.Now().UTC())
	} else {
		settled, err = pending.Reject(time.Now().UTC())
	}
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.decision_audit.settle_transition_failed", observability.Error(err))
		return
	}

	updateErr := a.uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		return a.factory.AgentDecisionRepository(db).UpdateSettlement(ctx, settled)
	})
	if updateErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.decision_audit.settle_failed",
			observability.String("decision_id", pending.ID().String()),
			observability.Error(updateErr),
		)
	}
}

func (a *decisionAuditor) redactResponse(ctx context.Context, directReply string, raw []byte) json.RawMessage {
	payload := directReply
	if payload == "" {
		payload = string(raw)
	}
	if payload == "" {
		return json.RawMessage(`{}`)
	}
	if a.redactor != nil {
		cleaned, cleanErr := a.redactor.Clean(payload)
		if cleanErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.decision_audit.redactor_failed",
				observability.Error(cleanErr),
			)
			return json.RawMessage(`{}`)
		}
		payload = cleaned
	}
	encoded, err := json.Marshal(struct {
		Redacted string `json:"redacted"`
	}{Redacted: payload})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

type decisionRecordInput struct {
	UserID       uuid.UUID
	Channel      string
	MessageID    string
	IntentKind   string
	PromptSHA256 string
	LLMModel     string
	TraceID      string
	DirectReply  string
	RawResponse  []byte
}
