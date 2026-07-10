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

type CardCreateConfirmContinuer struct {
	engine          workflow.Engine[workflows.CardCreateState]
	def             workflow.Definition[workflows.CardCreateState]
	threads         memory.ThreadGateway
	runs            agent.RunStore
	o11y            observability.Observability
	total           observability.Counter
	runUpdateErrors observability.Counter
}

func NewCardCreateConfirmContinuer(
	engine workflow.Engine[workflows.CardCreateState],
	def workflow.Definition[workflows.CardCreateState],
	threads memory.ThreadGateway,
	runs agent.RunStore,
	o11y observability.Observability,
) *CardCreateConfirmContinuer {
	total := o11y.Metrics().Counter(
		"agents_card_create_confirm_total",
		"Total de execucoes do gate de confirmacao de cadastro de cartao",
		"1",
	)
	return &CardCreateConfirmContinuer{
		engine:          engine,
		def:             def,
		threads:         threads,
		runs:            runs,
		o11y:            o11y,
		total:           total,
		runUpdateErrors: newRunUpdateErrorsCounter(o11y),
	}
}

func (c *CardCreateConfirmContinuer) Continue(ctx context.Context, resourceID, peer, message, messageID string) (bool, string, error) {
	ctx, span := c.o11y.Tracer().Start(ctx, "agents.usecase.card_create_confirm_continuer",
		observability.WithAttributes(
			observability.String("wamid", messageID),
		),
	)
	defer span.End()

	start := time.Now()

	run, runErr := c.openRun(ctx, resourceID, peer, messageID, start)
	if runErr != nil {
		span.RecordError(runErr)
		c.total.Add(ctx, 1, observability.String("outcome", "error"))
		return false, "", fmt.Errorf("agents.usecase.card_create_confirm_continuer: open_run: %w", runErr)
	}

	key := workflows.CardCreateKey(resourceID)

	patch, err := json.Marshal(map[string]string{
		"resumeText":        message,
		"incomingMessageId": messageID,
	})
	if err != nil {
		c.recordFailure(ctx, span, run, start, "marshal patch", err)
		return false, "", fmt.Errorf("agents.usecase.card_create_confirm_continuer: marshal patch: %w", err)
	}

	result, err := c.engine.Resume(ctx, c.def, key, patch)
	if err != nil {
		c.recordFailure(ctx, span, run, start, "resume", err)
		return false, "", fmt.Errorf("agents.usecase.card_create_confirm_continuer: resume: %w", err)
	}

	if result.Status == 0 {
		c.closeRun(ctx, run, agent.RunStatusSucceeded, "close", "", start)
		return false, "", nil
	}

	c.closeRun(ctx, run, agent.RunStatusSucceeded, "close", "", start)

	if result.Status == workflow.RunStatusSuspended {
		c.total.Add(ctx, 1, observability.String("outcome", "replied"))
		prompt := ""
		if result.Suspend != nil {
			prompt = result.Suspend.Prompt
		}
		return true, prompt, nil
	}

	if result.State.Expired {
		c.total.Add(ctx, 1, observability.String("outcome", "expired"))
		return false, "", nil
	}

	c.total.Add(ctx, 1, observability.String("outcome", cardCreateOutcomeString(result.State.Status)))
	return true, result.State.ResponseText, nil
}

func cardCreateOutcomeString(status workflows.CardCreateStatus) string {
	switch status {
	case workflows.CardCreateStatusCompleted:
		return "completed"
	case workflows.CardCreateStatusCancelled:
		return "cancelled"
	case workflows.CardCreateStatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (c *CardCreateConfirmContinuer) openRun(ctx context.Context, resourceID, threadID, wamid string, start time.Time) (agent.Run, error) {
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
		Workflow:         workflows.CardCreateConfirmWorkflowID,
		CorrelationKey:   wamid,
		Status:           agent.RunStatusRunning,
		StartedAt:        start.UTC(),
	}
	if err := c.runs.Insert(ctx, run); err != nil {
		return agent.Run{}, fmt.Errorf("insert_run: %w", err)
	}
	return run, nil
}

func (c *CardCreateConfirmContinuer) recordFailure(ctx context.Context, span observability.Span, run agent.Run, start time.Time, stage string, err error) {
	span.RecordError(err)
	span.SetStatus(observability.StatusCodeError, stage)
	c.total.Add(ctx, 1, observability.String("outcome", "error"))
	c.closeRun(ctx, run, agent.RunStatusFailed, stage, err.Error(), start)
	c.o11y.Logger().Error(ctx, "agents.usecase.card_create_confirm_continuer: "+stage+" falhou",
		observability.String("thread_id", run.ThreadID),
		observability.String("run_id", run.ID.String()),
		observability.String("wamid", run.CorrelationKey),
		observability.Error(err),
	)
}

func (c *CardCreateConfirmContinuer) closeRun(ctx context.Context, run agent.Run, status agent.RunStatus, stage, errStr string, start time.Time) {
	closeObservedRun(ctx, c.runs, c.o11y, c.runUpdateErrors, run, status, stage, errStr, start)
}
