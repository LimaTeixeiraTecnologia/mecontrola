package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PendingEntryContinuer struct {
	engine          workflow.Engine[workflows.PendingEntryState]
	def             workflow.Definition[workflows.PendingEntryState]
	threads         memory.ThreadGateway
	runs            agent.RunStore
	o11y            observability.Observability
	total           observability.Counter
	slotTotal       observability.Counter
	writeTotal      observability.Counter
	durationHist    observability.Histogram
	runUpdateErrors observability.Counter
}

func NewPendingEntryContinuer(
	engine workflow.Engine[workflows.PendingEntryState],
	def workflow.Definition[workflows.PendingEntryState],
	threads memory.ThreadGateway,
	runs agent.RunStore,
	o11y observability.Observability,
) *PendingEntryContinuer {
	total := o11y.Metrics().Counter(
		"agents_pending_entry_total",
		"Total de execucoes de retomada de pendencia de lancamento",
		"1",
	)
	slotTotal := o11y.Metrics().Counter(
		"agents_pending_entry_slot_total",
		"Total de slots fechados no fluxo de pendencia de lancamento",
		"1",
	)
	writeTotal := o11y.Metrics().Counter(
		"agents_pending_entry_write_total",
		"Total de escritas efetivadas pelo fluxo de pendencia de lancamento",
		"1",
	)
	durationHist := o11y.Metrics().Histogram(
		"agents_pending_entry_duration_seconds",
		"Duracao das execucoes de retomada de pendencia de lancamento",
		"s",
	)
	return &PendingEntryContinuer{
		engine:          engine,
		def:             def,
		threads:         threads,
		runs:            runs,
		o11y:            o11y,
		total:           total,
		slotTotal:       slotTotal,
		writeTotal:      writeTotal,
		durationHist:    durationHist,
		runUpdateErrors: newRunUpdateErrorsCounter(o11y),
	}
}

func (c *PendingEntryContinuer) Continue(ctx context.Context, userID, peer, message, messageID string) (workflows.PendingEntryResult, error) {
	ctx, span := c.o11y.Tracer().Start(ctx, "agents.usecase.pending_entry_continuer",
		observability.WithAttributes(
			observability.String("wamid", messageID),
		),
	)
	defer span.End()

	start := time.Now()

	run, runErr := c.openRun(ctx, userID, peer, messageID, start)
	if runErr != nil {
		span.RecordError(runErr)
		c.total.Add(ctx, 1, observability.String("outcome", "error"))
		c.durationHist.Record(ctx, time.Since(start).Seconds(), observability.String("outcome", "error"))
		return workflows.PendingEntryResult{}, fmt.Errorf("agents.usecase.pending_entry_continuer: open_run: %w", runErr)
	}

	key := workflows.PendingEntryKey(userID, peer)

	result, err := c.resolveResult(ctx, span, run, key, message, messageID, start)
	if err != nil {
		return workflows.PendingEntryResult{}, err
	}
	if result.Status == 0 {
		c.closeRun(ctx, run, agent.RunStatusSucceeded, "close", "", start)
		return workflows.PendingEntryResult{Handled: false}, nil
	}

	c.closeRun(ctx, run, agent.RunStatusSucceeded, "close", "", start)

	pendingResult := mapPendingEntryResult(result)
	outcome := pendingEntryModeString(pendingResult.Mode)

	c.total.Add(ctx, 1, observability.String("outcome", outcome))
	c.durationHist.Record(ctx, time.Since(start).Seconds(), observability.String("outcome", outcome))

	if result.Status == workflow.RunStatusSuspended {
		c.slotTotal.Add(ctx, 1,
			observability.String("slot", result.State.Awaiting.String()),
			observability.String("outcome", "replied"),
		)
	}

	if result.Status == workflow.RunStatusSucceeded {
		switch result.State.Status {
		case workflows.PendingStatusCompleted:
			c.slotTotal.Add(ctx, 1,
				observability.String("slot", "confirmation"),
				observability.String("outcome", "completed"),
			)
			c.writeTotal.Add(ctx, 1, observability.String("outcome", "success"))
		case workflows.PendingStatusCancelled:
			c.slotTotal.Add(ctx, 1,
				observability.String("slot", result.State.Awaiting.String()),
				observability.String("outcome", "cancelled"),
			)
		case workflows.PendingStatusExpired:
			c.slotTotal.Add(ctx, 1,
				observability.String("slot", result.State.Awaiting.String()),
				observability.String("outcome", "expired"),
			)
		}
	}

	return pendingResult, nil
}

func (c *PendingEntryContinuer) resolveResult(
	ctx context.Context,
	span observability.Span,
	run agent.Run,
	key, message, messageID string,
	start time.Time,
) (workflow.RunResult[workflows.PendingEntryState], error) {
	patch, err := json.Marshal(map[string]string{
		"resumeText":        message,
		"incomingMessageId": messageID,
	})
	if err != nil {
		c.recordFailure(ctx, span, run, start, "marshal patch", err)
		return workflow.RunResult[workflows.PendingEntryState]{}, fmt.Errorf("agents.usecase.pending_entry_continuer: marshal patch: %w", err)
	}

	result, err := c.engine.Resume(ctx, c.def, key, patch)
	if err != nil {
		c.recordFailure(ctx, span, run, start, "resume", err)
		return workflow.RunResult[workflows.PendingEntryState]{}, fmt.Errorf("agents.usecase.pending_entry_continuer: resume: %w", err)
	}
	if result.Status != 0 {
		return result, nil
	}

	revived, revivedErr := c.tryResumeFailedWrite(ctx, key, message, messageID)
	if revivedErr != nil {
		c.recordFailure(ctx, span, run, start, "resume_failed_write", revivedErr)
		return workflow.RunResult[workflows.PendingEntryState]{}, fmt.Errorf("agents.usecase.pending_entry_continuer: resume_failed_write: %w", revivedErr)
	}
	return revived, nil
}

func (c *PendingEntryContinuer) tryResumeFailedWrite(ctx context.Context, key, message, messageID string) (workflow.RunResult[workflows.PendingEntryState], error) {
	failedState, snap, found, err := c.engine.LoadLatestState(ctx, c.def, key)
	if err != nil {
		return workflow.RunResult[workflows.PendingEntryState]{}, fmt.Errorf("load_latest_state: %w", err)
	}
	if !found || snap.Status != workflow.RunStatusFailed {
		return workflow.RunResult[workflows.PendingEntryState]{}, nil
	}

	now := time.Now().UTC()
	if workflows.ShouldExpireAfterFailedWrite(failedState, now) {
		expired := workflows.SeedExpireAfterFailedWrite(failedState, workflows.PendingMessage{Text: message, MessageID: messageID})
		result, startErr := c.engine.Start(ctx, c.def, key, expired)
		if startErr != nil {
			return workflow.RunResult[workflows.PendingEntryState]{}, fmt.Errorf("start_expire: %w", startErr)
		}
		return result, nil
	}

	if !workflows.IsResumableAfterFailedWrite(failedState, now) {
		return workflow.RunResult[workflows.PendingEntryState]{}, nil
	}

	seeded := workflows.SeedResumeAfterFailedWrite(failedState, workflows.PendingMessage{Text: message, MessageID: messageID})

	result, startErr := c.engine.Start(ctx, c.def, key, seeded)
	if startErr != nil {
		return workflow.RunResult[workflows.PendingEntryState]{}, fmt.Errorf("start: %w", startErr)
	}
	return result, nil
}

func mapPendingEntryResult(result workflow.RunResult[workflows.PendingEntryState]) workflows.PendingEntryResult {
	if result.Status == workflow.RunStatusSuspended {
		prompt := ""
		if result.Suspend != nil {
			prompt = result.Suspend.Prompt
		}
		return workflows.PendingEntryResult{
			Handled: true,
			Message: prompt,
			Mode:    workflows.PendingEntryModeReplied,
		}
	}

	switch result.State.Status {
	case workflows.PendingStatusReplaced:
		return workflows.PendingEntryResult{
			Handled: false,
			Mode:    workflows.PendingEntryModeReplaced,
		}
	case workflows.PendingStatusCompleted:
		return workflows.PendingEntryResult{
			Handled: true,
			Message: result.State.ResponseText,
			Mode:    workflows.PendingEntryModeCompleted,
		}
	case workflows.PendingStatusCancelled:
		return workflows.PendingEntryResult{
			Handled: true,
			Message: result.State.ResponseText,
			Mode:    workflows.PendingEntryModeCancelled,
		}
	case workflows.PendingStatusExpired:
		return workflows.PendingEntryResult{
			Handled: true,
			Message: result.State.ResponseText,
			Mode:    workflows.PendingEntryModeExpired,
		}
	default:
		return workflows.PendingEntryResult{
			Handled: true,
			Message: result.State.ResponseText,
			Mode:    workflows.PendingEntryModeReplied,
		}
	}
}

func pendingEntryModeString(mode workflows.PendingEntryMode) string {
	switch mode {
	case workflows.PendingEntryModeReplied:
		return "replied"
	case workflows.PendingEntryModePassThrough:
		return "pass_through"
	case workflows.PendingEntryModeCompleted:
		return "completed"
	case workflows.PendingEntryModeCancelled:
		return "cancelled"
	case workflows.PendingEntryModeExpired:
		return "expired"
	case workflows.PendingEntryModeReplaced:
		return "replaced"
	default:
		return "unknown"
	}
}

func (c *PendingEntryContinuer) openRun(ctx context.Context, resourceID, threadID, wamid string, start time.Time) (agent.Run, error) {
	thread, err := c.threads.GetOrCreate(ctx, resourceID, threadID)
	if err != nil {
		return agent.Run{}, fmt.Errorf("get_or_create_thread: %w", err)
	}

	run := agent.Run{
		ID:               uuid.New(),
		PlatformThreadID: thread.ID,
		ResourceID:       resourceID,
		ThreadID:         threadID,
		AgentID:          mecontrolaAgentID,
		Workflow:         workflows.PendingEntryWorkflowID,
		CorrelationKey:   wamid,
		Status:           agent.RunStatusRunning,
		StartedAt:        start.UTC(),
	}
	if err := c.runs.Insert(ctx, run); err != nil {
		return agent.Run{}, fmt.Errorf("insert_run: %w", err)
	}
	return run, nil
}

func (c *PendingEntryContinuer) recordFailure(ctx context.Context, span observability.Span, run agent.Run, start time.Time, stage string, err error) {
	span.RecordError(err)
	span.SetStatus(observability.StatusCodeError, stage)
	c.total.Add(ctx, 1, observability.String("outcome", "error"))
	c.durationHist.Record(ctx, time.Since(start).Seconds(), observability.String("outcome", "error"))
	c.closeRun(ctx, run, agent.RunStatusFailed, stage, err.Error(), start)
	c.o11y.Logger().Error(ctx, "agents.usecase.pending_entry_continuer: "+stage+" falhou",
		observability.String("thread_id", run.ThreadID),
		observability.String("run_id", run.ID.String()),
		observability.String("wamid", run.CorrelationKey),
		observability.Error(err),
	)
}

func (c *PendingEntryContinuer) closeRun(ctx context.Context, run agent.Run, status agent.RunStatus, stage, errStr string, start time.Time) {
	closeObservedRun(ctx, c.runs, c.o11y, c.runUpdateErrors, run, status, stage, errStr, start)
}
