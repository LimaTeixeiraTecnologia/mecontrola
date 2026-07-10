package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type runtimeMetrics struct {
	runsTotal          observability.Counter
	runDuration        observability.Histogram
	runTruncatedTotal  observability.Counter
	runUpdateErrors    observability.Counter
	messageAppendError observability.Counter
}

type agentRuntime struct {
	agents       AgentRegistry
	threads      memory.ThreadGateway
	messages     memory.MessageStore
	workingMem   memory.WorkingMemory
	runs         RunStore
	hooks        Hooks
	o11y         observability.Observability
	metrics      runtimeMetrics
	writeToolSet map[string]struct{}
}

type RuntimeOption func(*agentRuntime)

func WithRuntimeHooks(h Hooks) RuntimeOption {
	return func(r *agentRuntime) {
		r.hooks = h
	}
}

func WithWriteToolSet(names ...string) RuntimeOption {
	return func(r *agentRuntime) {
		if r.writeToolSet == nil {
			r.writeToolSet = make(map[string]struct{}, len(names))
		}
		for _, n := range names {
			r.writeToolSet[n] = struct{}{}
		}
	}
}

func NewAgentRuntime(
	agents AgentRegistry,
	threads memory.ThreadGateway,
	messages memory.MessageStore,
	workingMem memory.WorkingMemory,
	runs RunStore,
	o11y observability.Observability,
	opts ...RuntimeOption,
) AgentRuntime {
	r := &agentRuntime{
		agents:     agents,
		threads:    threads,
		messages:   messages,
		workingMem: workingMem,
		runs:       runs,
		hooks:      NoopHooks{},
		o11y:       o11y,
		metrics: runtimeMetrics{
			runsTotal:          o11y.Metrics().Counter("agent_runs_total", "Total agent runs", "1"),
			runDuration:        o11y.Metrics().Histogram("agent_run_duration_seconds", "Agent run duration", "s"),
			runTruncatedTotal:  o11y.Metrics().Counter("agent_run_truncated_total", "Total agent runs truncated by length", "1"),
			runUpdateErrors:    o11y.Metrics().Counter("agent_run_update_errors_total", "Total RunStore.Update failures", "1"),
			messageAppendError: o11y.Metrics().Counter("agent_message_append_errors_total", "Total MessageStore.Append failures", "1"),
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *agentRuntime) Execute(ctx context.Context, in InboundRequest) (Outcome, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.runtime.execute",
		observability.WithAttributes(
			observability.String("agent_id", in.AgentID),
		),
	)
	defer span.End()

	if err := in.Validate(); err != nil {
		return Outcome{}, err
	}

	start := time.Now()

	thread, err := r.threads.GetOrCreate(ctx, in.ResourceID, in.ThreadID)
	if err != nil {
		span.RecordError(err)
		return Outcome{}, fmt.Errorf("agent.runtime.execute: get_or_create_thread: %w", err)
	}

	runID := uuid.New()
	run := Run{
		ID:               runID,
		PlatformThreadID: thread.ID,
		ResourceID:       in.ResourceID,
		ThreadID:         in.ThreadID,
		AgentID:          in.AgentID,
		CorrelationKey:   in.MessageID,
		Status:           RunStatusRunning,
		StartedAt:        time.Now().UTC(),
	}
	if err := r.runs.Insert(ctx, run); err != nil {
		span.RecordError(err)
		return Outcome{}, fmt.Errorf("agent.runtime.execute: insert_run: %w", err)
	}

	ctx = WithRunID(workflow.WithRuntime(ctx, in), runID)
	ctx = withToolIdentity(ctx, in)

	a, err := r.agents.Resolve(in.AgentID)
	if err != nil {
		r.failRun(ctx, span, run, ToolOutcomeMissingResolver, "resolve_agent", err, start)
		return Outcome{}, fmt.Errorf("agent.runtime.execute: resolve_agent: %w", err)
	}

	msgs, err := r.buildMessages(ctx, a, thread.ID, in)
	if err != nil {
		r.failRun(ctx, span, run, ToolOutcomeUsecaseError, "build_messages", err, start)
		return Outcome{}, fmt.Errorf("agent.runtime.execute: build_messages: %w", err)
	}

	req := Request{
		AgentID:    in.AgentID,
		ResourceID: in.ResourceID,
		ThreadID:   in.ThreadID,
		Messages:   msgs,
	}

	ctx = r.hooks.BeforeExecute(ctx, in.AgentID, req)

	result, execErr := a.Execute(ctx, req)

	r.hooks.AfterExecute(ctx, in.AgentID, result, execErr)

	if execErr != nil {
		r.failRun(ctx, span, run, ToolOutcomeUsecaseError, "agent.execute", execErr, start)
		return Outcome{}, fmt.Errorf("agent.runtime.execute: agent.execute: %w", execErr)
	}

	return r.finishRun(ctx, run, thread.ID, in, result, start), nil
}

func (r *agentRuntime) failRun(ctx context.Context, span observability.Span, run Run, outcome ToolOutcome, stage string, err error, start time.Time) {
	r.closeRun(ctx, run, RunStatusFailed, outcome, err.Error(), start)
	r.logRunError(ctx, run, stage, err)
	span.RecordError(err)
	span.SetStatus(observability.StatusCodeError, stage)
}

func (r *agentRuntime) logRunError(ctx context.Context, run Run, stage string, err error) {
	r.o11y.Logger().Error(ctx, "agent.runtime.execute: "+stage+" falhou",
		observability.String("agent_id", run.AgentID),
		observability.Error(err),
	)
}

func (r *agentRuntime) finishRun(ctx context.Context, run Run, platformThreadID uuid.UUID, in InboundRequest, result Result, start time.Time) Outcome {
	r.appendMessage(ctx, platformThreadID, in, memory.Message{
		ID:               uuid.New(),
		PlatformThreadID: platformThreadID,
		ResourceID:       in.ResourceID,
		Role:             memory.RoleUser,
		Content:          in.Message,
		CreatedAt:        time.Now().UTC(),
	})
	r.appendMessage(ctx, platformThreadID, in, memory.Message{
		ID:               uuid.New(),
		PlatformThreadID: platformThreadID,
		ResourceID:       in.ResourceID,
		Role:             memory.RoleAssistant,
		Content:          result.Content,
		CreatedAt:        time.Now().UTC(),
	})

	toolOutcome := ToolOutcomeRouted
	if result.ToolOutcome == ToolOutcomeUsecaseError {
		toolOutcome = ToolOutcomeUsecaseError
	}
	runStatus := RunStatusSucceeded
	errStr := ""
	if toolOutcome == ToolOutcomeUsecaseError || strings.TrimSpace(result.Content) == "" {
		runStatus = RunStatusFailed
		toolOutcome = ToolOutcomeUsecaseError
		errStr = aggregateToolErrorContent(result.ToolCalls)
	}
	if runStatus == RunStatusSucceeded && r.writeToolGuardFailed(result.ToolCalls) {
		runStatus = RunStatusFailed
		toolOutcome = ToolOutcomeUsecaseError
		errStr = aggregateToolErrorContent(result.ToolCalls)
	}
	if result.TruncatedByLength {
		runStatus = RunStatusFailed
		toolOutcome = ToolOutcomeTruncated
		errStr = "resposta truncada por length"
		r.metrics.runTruncatedTotal.Add(ctx, 1, observability.String("agent_id", run.AgentID))
	}

	if runStatus == RunStatusFailed {
		r.recordRunFailure(ctx, run, errStr)
	}

	r.closeRun(ctx, run, runStatus, toolOutcome, errStr, start)

	content := result.Content
	if runStatus != RunStatusSucceeded {
		content = ""
	}

	return Outcome{
		RunID:   run.ID,
		Content: content,
		Status:  runStatus,
		Outcome: toolOutcome,
		Mode:    ExecutionModeSync,
	}
}

func (r *agentRuntime) appendMessage(ctx context.Context, platformThreadID uuid.UUID, in InboundRequest, msg memory.Message) {
	if err := r.messages.Append(ctx, platformThreadID, msg); err != nil {
		r.metrics.messageAppendError.Add(ctx, 1,
			observability.String("agent_id", in.AgentID),
			observability.String("role", msg.Role.String()),
		)
		r.o11y.Logger().Warn(ctx, "agent.runtime.finish_run: append message falhou",
			observability.String("agent_id", in.AgentID),
			observability.String("role", msg.Role.String()),
			observability.Error(err),
		)
	}
}

const maxAggregatedToolErrors = 3

func aggregateToolErrorContent(calls []ToolCallRecord) string {
	var errs []string
	for _, c := range calls {
		if c.Outcome != ToolCallOutcomeError {
			continue
		}
		content := strings.TrimSpace(c.Content)
		if content == "" {
			continue
		}
		errs = append(errs, content)
		if len(errs) >= maxAggregatedToolErrors {
			break
		}
	}
	return strings.Join(errs, " | ")
}

func (r *agentRuntime) recordRunFailure(ctx context.Context, run Run, errStr string) {
	_, span := r.o11y.Tracer().Start(ctx, "agents.runtime.tool_call_failure",
		observability.WithAttributes(
			observability.String("agent_id", run.AgentID),
		),
	)
	defer span.End()
	if errStr != "" {
		span.RecordError(fmt.Errorf("agent.runtime.finish_run: %s", errStr))
	}
	span.SetStatus(observability.StatusCodeError, "tool_call_failure")

	r.o11y.Logger().Error(ctx, "agent.runtime.finish_run: falha na execucao do run",
		observability.String("agent_id", run.AgentID),
		observability.String("error", errStr),
	)
}

func (r *agentRuntime) writeToolGuardFailed(calls []ToolCallRecord) bool {
	if len(r.writeToolSet) == 0 {
		return false
	}
	wroteAtLeastOne := false
	for _, c := range calls {
		if _, ok := r.writeToolSet[c.Tool]; ok {
			wroteAtLeastOne = true
			if c.Outcome == ToolCallOutcomeSuccess {
				return false
			}
		}
	}
	return wroteAtLeastOne
}

func (r *agentRuntime) buildMessages(ctx context.Context, a Agent, threadPK uuid.UUID, in InboundRequest) ([]llm.Message, error) {
	var msgs []llm.Message

	instructions := a.Instructions()
	wm, _ := r.workingMem.Get(ctx, in.ResourceID)
	systemContent := instructions
	if wm != "" {
		systemContent = instructions + "\n\n## Working Memory\n" + wm
	}
	if systemContent != "" {
		msgs = append(msgs, llm.Message{Role: "system", Content: systemContent})
	}

	recent, _ := r.messages.Recent(ctx, threadPK, 20)
	for i := len(recent) - 1; i >= 0; i-- {
		m := recent[i]
		if m.Role == memory.RoleTool {
			continue
		}
		msgs = append(msgs, llm.Message{Role: m.Role.String(), Content: m.Content})
	}

	msgs = append(msgs, llm.Message{Role: "user", Content: in.Message})
	return msgs, nil
}

func (r *agentRuntime) closeRun(ctx context.Context, run Run, status RunStatus, outcome ToolOutcome, errStr string, start time.Time) {
	now := time.Now().UTC()
	run.Status = status
	run.Outcome = outcome
	run.Error = errStr
	run.EndedAt = &now
	run.DurationMs = time.Since(start).Milliseconds()

	if err := r.runs.Update(ctx, run); err != nil {
		r.metrics.runUpdateErrors.Add(ctx, 1, observability.String("agent_id", run.AgentID))
		r.o11y.Logger().Error(ctx, "agent.runtime.close_run: run_store.update falhou",
			observability.String("agent_id", run.AgentID),
			observability.Error(err),
		)
		r.metrics.runDuration.Record(ctx, time.Since(start).Seconds(),
			observability.String("agent_id", run.AgentID),
		)
		return
	}

	r.metrics.runsTotal.Add(ctx, 1,
		observability.String("agent_id", run.AgentID),
		observability.String("status", status.String()),
	)
	r.metrics.runDuration.Record(ctx, time.Since(start).Seconds(),
		observability.String("agent_id", run.AgentID),
	)
}
