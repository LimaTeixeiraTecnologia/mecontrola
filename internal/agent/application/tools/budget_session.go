package tools

import (
	"context"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type BudgetCommitGateFunc func(ctx context.Context, userID uuid.UUID, channel, messageID string, draft budgetdraft.Draft) (gated bool, result ToolResult)

type BudgetSessionRunner struct {
	recorder   *Recorder
	session    BudgetSessionGateway
	convo      BudgetConversation
	committer  BudgetConfigCommitter
	commitGate BudgetCommitGateFunc
	loc        *time.Location
	o11y       observability.Observability
}

func NewBudgetSessionRunner(recorder *Recorder, session BudgetSessionGateway, convo BudgetConversation, committer BudgetConfigCommitter, loc *time.Location, o11y observability.Observability) *BudgetSessionRunner {
	return &BudgetSessionRunner{recorder: recorder, session: session, convo: convo, committer: committer, loc: loc, o11y: o11y}
}

func (r *BudgetSessionRunner) WithCommitGate(gate BudgetCommitGateFunc) {
	r.commitGate = gate
}

func (r *BudgetSessionRunner) Enabled() bool {
	return r != nil && r.session != nil && r.convo != nil && r.committer != nil
}

func (r *BudgetSessionRunner) Start(ctx context.Context, userID uuid.UUID, channel, text, messageID string) ToolResult {
	return r.advance(ctx, userID, channel, text, messageID, budgetdraft.New(currentCompetence(r.loc)))
}

func (r *BudgetSessionRunner) Continue(ctx context.Context, userID uuid.UUID, channel, text, messageID string) (bool, ToolResult) {
	draft, found, err := r.session.Load(ctx, userID, channel)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_load_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return false, ToolResult{}
	}
	if !found {
		return false, ToolResult{}
	}
	if matchesBudgetCancel(text) {
		if clearErr := r.session.Clear(ctx, userID, channel); clearErr != nil {
			r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
				observability.String("channel", channel),
				observability.Error(clearErr),
			)
		}
		r.recorder.Record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
		return true, ToolResult{Reply: budgetCancelledText, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
	}
	return true, r.advance(ctx, userID, channel, text, messageID, draft)
}

func (r *BudgetSessionRunner) advance(ctx context.Context, userID uuid.UUID, channel, text, messageID string, draft budgetdraft.Draft) ToolResult {
	result, err := r.convo.Configure(ctx, text, draft)
	if err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_configure_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		r.recorder.Record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}

	if !result.Complete {
		if saveErr := r.session.Save(ctx, userID, channel, result.Draft); saveErr != nil {
			r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_save_failed",
				observability.String("channel", channel),
				observability.Error(saveErr),
			)
			r.recorder.Record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
			return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
		}
		r.recorder.Record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
		return ToolResult{Reply: result.Reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
	}

	if r.commitGate != nil {
		if gated, gateResult := r.commitGate(ctx, userID, channel, messageID, result.Draft); gated {
			return gateResult
		}
	}

	reply, commitErr := r.committer.Commit(ctx, userID, result.Draft)
	if commitErr != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_commit_failed",
			observability.String("channel", channel),
			observability.Error(commitErr),
		)
		if saveErr := r.session.Save(ctx, userID, channel, result.Draft); saveErr != nil {
			r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_save_failed",
				observability.String("channel", channel),
				observability.Error(saveErr),
			)
		}
		r.recorder.Record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return ToolResult{Reply: reply, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}

	if clearErr := r.session.Clear(ctx, userID, channel); clearErr != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
			observability.String("channel", channel),
			observability.Error(clearErr),
		)
	}
	r.recorder.Record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
	return ToolResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
}

func budgetDefaultStartReply(reply string) string {
	if strings.TrimSpace(reply) == "" {
		return "Beleza! Vamos montar seu plano. Qual é o seu objetivo financeiro principal?"
	}
	return reply
}
