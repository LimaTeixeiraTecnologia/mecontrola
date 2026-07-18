package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

var ErrUnknownSuspendedWorkflow = errors.New("usecases.resume_dispatcher: workflow suspenso sem resumer registrado")

type ResumeDispatcher struct {
	index               *SuspendedRunIndex
	resumers            map[string]WorkflowResumer
	threads             memory.ThreadGateway
	runs                agent.RunStore
	messages            memory.MessageStore
	o11y                observability.Observability
	total               observability.Counter
	durationHist        observability.Histogram
	runUpdateErrors     observability.Counter
	messageAppendErrors observability.Counter
}

func NewResumeDispatcher(
	index *SuspendedRunIndex,
	threads memory.ThreadGateway,
	runs agent.RunStore,
	messages memory.MessageStore,
	o11y observability.Observability,
	resumers ...WorkflowResumer,
) (*ResumeDispatcher, error) {
	m := make(map[string]WorkflowResumer, len(resumers))
	for _, r := range resumers {
		if _, exists := m[r.WorkflowID()]; exists {
			return nil, fmt.Errorf("usecases.resume_dispatcher: workflow %s registrado mais de uma vez", r.WorkflowID())
		}
		m[r.WorkflowID()] = r
	}

	total := o11y.Metrics().Counter(
		"agents_resume_dispatch_total",
		"Total de despachos de resume por workflow suspenso",
		"1",
	)
	durationHist := o11y.Metrics().Histogram(
		"agents_resume_dispatch_duration_seconds",
		"Duracao do despacho de resume de workflow suspenso",
		"s",
	)

	return &ResumeDispatcher{
		index:           index,
		resumers:        m,
		threads:         threads,
		runs:            runs,
		messages:        messages,
		o11y:            o11y,
		total:           total,
		durationHist:    durationHist,
		runUpdateErrors: newRunUpdateErrorsCounter(o11y),
		messageAppendErrors: o11y.Metrics().Counter(
			"agents_resume_message_append_errors_total",
			"Total de falhas ao anexar mensagens de resume na thread",
			"1",
		),
	}, nil
}

func (d *ResumeDispatcher) Continue(ctx context.Context, resourceID, threadID, message, messageID string) (bool, string, error) {
	ctx, span := d.o11y.Tracer().Start(ctx, "agents.usecase.resume_dispatcher",
		observability.WithAttributes(
			observability.String("wamid", messageID),
		),
	)
	defer span.End()

	start := time.Now()

	workflowID, found, err := d.index.Resolve(ctx, resourceID, threadID)
	if err != nil {
		span.RecordError(err)
		d.total.Add(ctx, 1, observability.String("outcome", "index_error"))
		return false, "", fmt.Errorf("usecases.resume_dispatcher: resolve: %w", err)
	}
	if !found {
		return false, "", nil
	}

	resumer, known := d.resumers[workflowID]
	if !known {
		unknownErr := fmt.Errorf("%w: %s", ErrUnknownSuspendedWorkflow, workflowID)
		span.RecordError(unknownErr)
		d.total.Add(ctx, 1,
			observability.String("outcome", "unknown_workflow"),
			observability.String("workflow", workflowID),
		)
		return false, "", unknownErr
	}

	run, runErr := d.openRun(ctx, resourceID, threadID, messageID, workflowID, start)
	if runErr != nil {
		span.RecordError(runErr)
		d.total.Add(ctx, 1,
			observability.String("outcome", "run_error"),
			observability.String("workflow", workflowID),
		)
		return false, "", fmt.Errorf("usecases.resume_dispatcher: open_run: %w", runErr)
	}

	ctx = agent.WithToolInvocationContext(ctx, resourceID, messageID, 0)

	handled, reply, resumeErr := resumer.Resume(ctx, resourceID, threadID, message, messageID)
	if resumeErr != nil {
		span.RecordError(resumeErr)
		d.total.Add(ctx, 1,
			observability.String("outcome", "resume_error"),
			observability.String("workflow", workflowID),
		)
		d.durationHist.Record(ctx, time.Since(start).Seconds(),
			observability.String("workflow", workflowID),
			observability.String("outcome", "resume_error"),
		)
		d.closeRun(ctx, run, agent.RunStatusFailed, "resume", resumeErr.Error(), start)
		if handled && reply != "" {
			d.appendExchange(ctx, run, workflowID, message, reply)
		}
		return handled, reply, fmt.Errorf("usecases.resume_dispatcher: resume: %w", resumeErr)
	}

	outcome := "not_handled"
	if handled {
		outcome = "handled"
	}
	d.total.Add(ctx, 1,
		observability.String("outcome", outcome),
		observability.String("workflow", workflowID),
	)
	d.durationHist.Record(ctx, time.Since(start).Seconds(),
		observability.String("workflow", workflowID),
		observability.String("outcome", outcome),
	)
	d.closeRun(ctx, run, agent.RunStatusSucceeded, "close", "", start)

	if handled && reply != "" {
		d.appendExchange(ctx, run, workflowID, message, reply)
	}

	return handled, reply, nil
}

func (d *ResumeDispatcher) appendExchange(ctx context.Context, run agent.Run, workflowID, userMessage, reply string) {
	if d.messages == nil {
		return
	}
	pairs := []memory.Message{
		{
			ID:               uuid.New(),
			PlatformThreadID: run.PlatformThreadID,
			ResourceID:       run.ResourceID,
			Role:             memory.RoleUser,
			Content:          userMessage,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               uuid.New(),
			PlatformThreadID: run.PlatformThreadID,
			ResourceID:       run.ResourceID,
			Role:             memory.RoleAssistant,
			Content:          reply,
			CreatedAt:        time.Now().UTC(),
		},
	}
	for _, msg := range pairs {
		if err := d.messages.Append(ctx, run.PlatformThreadID, msg); err != nil {
			d.messageAppendErrors.Add(ctx, 1,
				observability.String("workflow", workflowID),
				observability.String("role", msg.Role.String()),
			)
			d.o11y.Logger().Warn(ctx, "agents.usecase.resume_dispatcher: append message falhou",
				observability.String("workflow", workflowID),
				observability.String("role", msg.Role.String()),
				observability.Error(err),
			)
		}
	}
}

func (d *ResumeDispatcher) openRun(ctx context.Context, resourceID, threadID, wamid, workflowID string, start time.Time) (agent.Run, error) {
	thread, err := d.threads.GetOrCreate(ctx, resourceID, threadID)
	if err != nil {
		return agent.Run{}, fmt.Errorf("get_or_create_thread: %w", err)
	}

	run := agent.Run{
		ID:               uuid.New(),
		PlatformThreadID: thread.ID,
		ResourceID:       resourceID,
		ThreadID:         threadID,
		AgentID:          mecontrolaAgentID,
		Workflow:         workflowID,
		CorrelationKey:   wamid,
		Status:           agent.RunStatusRunning,
		StartedAt:        start.UTC(),
	}
	if err := d.runs.Insert(ctx, run); err != nil {
		return agent.Run{}, fmt.Errorf("insert_run: %w", err)
	}
	return run, nil
}

func (d *ResumeDispatcher) closeRun(ctx context.Context, run agent.Run, status agent.RunStatus, stage, errStr string, start time.Time) {
	closeObservedRun(ctx, d.runs, d.o11y, d.runUpdateErrors, run, status, stage, errStr, start)
}
