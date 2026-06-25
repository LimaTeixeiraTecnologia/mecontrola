package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
)

const humanConfirmationLLMModel = "human_confirmation"

var humanConfirmationPromptSHA256 = func() string {
	sum := sha256.Sum256([]byte("human_confirmation"))
	return hex.EncodeToString(sum[:])
}()

func NewConfirmReplayFunc(o11y observability.Observability, deps DecisionAuditDeps, redactor DecisionRedactor) steps.ConfirmReplayFunc {
	auditor := newDecisionAuditor(o11y, deps, redactor)
	return func(ctx context.Context, state confirmation.ConfirmState) (string, bool) {
		if !auditor.enabled() || state.ResumeMessageID == "" {
			return "", false
		}
		uid, err := uuid.Parse(state.UserID)
		if err != nil {
			return "", false
		}
		reply, found := auditor.lookup(ctx, uid, state.Channel, state.ResumeMessageID, state.StepIndex)
		if !found {
			return "", false
		}
		return reply, true
	}
}

func NewConfirmAuditBeginFunc(o11y observability.Observability, deps DecisionAuditDeps, redactor DecisionRedactor) steps.ConfirmAuditBeginFunc {
	auditor := newDecisionAuditor(o11y, deps, redactor)
	return func(ctx context.Context, state confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
		if !auditor.enabled() {
			return steps.ConfirmAuditBeginResult{}
		}
		messageID := state.ResumeMessageID
		if messageID == "" {
			messageID = state.MessageID
		}
		if messageID == "" {
			return steps.ConfirmAuditBeginResult{}
		}
		uid, err := uuid.Parse(state.UserID)
		if err != nil {
			return steps.ConfirmAuditBeginResult{}
		}
		traceID := ""
		if span := o11y.Tracer().SpanFromContext(ctx); span != nil {
			traceID = span.TraceID()
		}
		dctx := auditor.beginHuman(ctx, decisionRecordInput{
			UserID:       uid,
			Channel:      state.Channel,
			MessageID:    messageID,
			IntentKind:   state.OperationKind.String(),
			PromptSHA256: humanConfirmationPromptSHA256,
			LLMModel:     humanConfirmationLLMModel,
			TraceID:      traceID,
			DirectReply:  "",
			RawResponse:  nil,
			StepIndex:    state.StepIndex,
		})
		settle := steps.ConfirmAuditSettleFunc(nil)
		if dctx.recorded {
			settle = func(ctx context.Context, executed bool) {
				dctx.settle(ctx, executed)
			}
		}
		return steps.ConfirmAuditBeginResult{
			Conflicted: dctx.conflicted,
			Failed:     dctx.failed,
			Settle:     settle,
			DecisionID: dctx.pending.ID(),
		}
	}
}
