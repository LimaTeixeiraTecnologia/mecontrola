package services

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

const (
	runtimeAgentID         = "daily_agent"
	workflowTransactions   = "transactions"
	workflowBudget         = "budget"
	workflowCards          = "cards"
	workflowConversational = "conversational"
)

type runtimeRouter interface {
	route(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult
}

type ThreadGateway interface {
	GetOrCreate(ctx context.Context, userID uuid.UUID, channel string) (entities.Thread, error)
}

type RunGateway interface {
	Insert(ctx context.Context, run entities.Run) error
	Finish(ctx context.Context, run entities.Run) error
}

type AgentRuntime struct {
	router          runtimeRouter
	threads         ThreadGateway
	runs            RunGateway
	o11y            observability.Observability
	runsTotal       observability.Counter
	runDuration     observability.Histogram
	toolInvocations observability.Counter
}

func NewAgentRuntime(o11y observability.Observability, router runtimeRouter, threads ThreadGateway, runs RunGateway) *AgentRuntime {
	runsTotal := o11y.Metrics().Counter(
		"agent_runs_total",
		"Total de runs do AgentRuntime por agent_id, channel, workflow e status",
		"1",
	)
	runDuration := o11y.Metrics().Histogram(
		"agent_run_duration_seconds",
		"Duracao das runs do AgentRuntime por agent_id, channel e workflow",
		"s",
	)
	toolInvocations := o11y.Metrics().Counter(
		"agent_tool_invocations_total",
		"Total de invocacoes de tool por tool e outcome",
		"1",
	)
	return &AgentRuntime{
		router:          router,
		threads:         threads,
		runs:            runs,
		o11y:            o11y,
		runsTotal:       runsTotal,
		runDuration:     runDuration,
		toolInvocations: toolInvocations,
	}
}

func (rt *AgentRuntime) Execute(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	ctx, span := rt.o11y.Tracer().Start(ctx, "agent.runtime.execute")
	defer span.End()

	run, hasRun := rt.startRun(ctx, principal, channel, messageID)

	result := rt.router.route(ctx, principal, channel, peer, text, messageID)

	workflow := workflowFor(result.Kind)
	tool := toolFor(result.Kind)
	ok := outcomeSucceeded(result.Outcome)

	if hasRun {
		resolved := run.Resolve(entities.RunResolution{
			Workflow:   workflow,
			ToolName:   tool,
			IntentKind: result.Kind.String(),
		})
		finished := resolved.Finish(result.Outcome, ok, runErrText(ok, result.Outcome))
		rt.finishRun(ctx, finished)
		span.SetAttributes(
			observability.String("run_id", finished.ID().String()),
			observability.String("thread_id", finished.ThreadID().String()),
			observability.String("workflow", workflow),
			observability.String("tool", tool),
			observability.String("outcome", result.Outcome),
			observability.String("status", finished.Status().String()),
			observability.Int64("duration_ms", finished.DurationMs()),
		)
		rt.recordMetrics(ctx, channel, workflow, tool, result.Outcome, finished.Status(), finished.DurationMs())
	} else {
		status := entities.RunStatusFailed
		if ok {
			status = entities.RunStatusSucceeded
		}
		span.SetAttributes(
			observability.String("workflow", workflow),
			observability.String("tool", tool),
			observability.String("outcome", result.Outcome),
			observability.String("status", status.String()),
		)
		rt.recordMetrics(ctx, channel, workflow, tool, result.Outcome, status, 0)
	}

	span.SetAttributes(observability.String("kind", result.Kind.String()))
	return result
}

func (rt *AgentRuntime) startRun(ctx context.Context, principal Principal, channel, messageID string) (entities.Run, bool) {
	thread, err := rt.threads.GetOrCreate(ctx, principal.UserID, channel)
	if err != nil {
		rt.o11y.Logger().Warn(ctx, "agent.runtime.thread_resolve_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return entities.Run{}, false
	}

	run, err := entities.StartRun(entities.StartRunParams{
		ThreadID:  thread.ID(),
		UserID:    principal.UserID,
		Channel:   channel,
		MessageID: messageID,
		AgentID:   runtimeAgentID,
	})
	if err != nil {
		rt.o11y.Logger().Warn(ctx, "agent.runtime.start_run_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return entities.Run{}, false
	}

	if insertErr := rt.runs.Insert(ctx, run); insertErr != nil {
		rt.o11y.Logger().Warn(ctx, "agent.runtime.run_insert_failed",
			observability.String("run_id", run.ID().String()),
			observability.Error(insertErr),
		)
		return entities.Run{}, false
	}
	return run, true
}

func (rt *AgentRuntime) finishRun(ctx context.Context, run entities.Run) {
	if err := rt.runs.Finish(ctx, run); err != nil {
		rt.o11y.Logger().Warn(ctx, "agent.runtime.run_finish_failed",
			observability.String("run_id", run.ID().String()),
			observability.Error(err),
		)
	}
}

func (rt *AgentRuntime) recordMetrics(ctx context.Context, channel, workflow, tool, outcome string, status entities.RunStatus, durationMs int64) {
	rt.runsTotal.Add(ctx, 1,
		observability.String("agent_id", runtimeAgentID),
		observability.String("channel", channel),
		observability.String("workflow", workflow),
		observability.String("status", status.String()),
	)
	if durationMs > 0 {
		rt.runDuration.Record(ctx, float64(durationMs)/1000.0,
			observability.String("agent_id", runtimeAgentID),
			observability.String("channel", channel),
			observability.String("workflow", workflow),
		)
	}
	if tool != "" {
		rt.toolInvocations.Add(ctx, 1,
			observability.String("tool", tool),
			observability.String("outcome", outcome),
		)
	}
}

func outcomeSucceeded(outcome string) bool {
	switch outcome {
	case OutcomeRouted,
		OutcomeReplay,
		OutcomeFallback,
		OutcomeClarify,
		OutcomeAuthzDenied,
		OutcomePolicyBlocked,
		OutcomeEmptyText:
		return true
	default:
		return false
	}
}

func runErrText(ok bool, outcome string) string {
	if ok {
		return ""
	}
	return outcome
}

func workflowFor(kind intent.Kind) string {
	switch kind {
	case intent.KindRecordExpense,
		intent.KindRecordIncome,
		intent.KindRecordCardPurchase,
		intent.KindListTransactions,
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindCreateRecurring,
		intent.KindListRecurring:
		return workflowTransactions
	case intent.KindMonthlySummary,
		intent.KindHowAmIDoing,
		intent.KindConfigureBudget,
		intent.KindEditCategoryPercentage,
		intent.KindQueryCategory,
		intent.KindQueryGoal:
		return workflowBudget
	case intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindUpdateCard,
		intent.KindDeleteCard,
		intent.KindQueryCard:
		return workflowCards
	default:
		return workflowConversational
	}
}

func toolFor(kind intent.Kind) string {
	switch workflowFor(kind) {
	case workflowConversational:
		return ""
	default:
		return kind.String()
	}
}
