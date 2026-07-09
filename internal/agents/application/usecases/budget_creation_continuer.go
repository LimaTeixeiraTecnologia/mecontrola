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

type BudgetCreationContinuer struct {
	engine  workflow.Engine[workflows.BudgetCreationState]
	def     workflow.Definition[workflows.BudgetCreationState]
	threads memory.ThreadGateway
	runs    agent.RunStore
	o11y    observability.Observability
	total   observability.Counter
}

func NewBudgetCreationContinuer(
	engine workflow.Engine[workflows.BudgetCreationState],
	def workflow.Definition[workflows.BudgetCreationState],
	threads memory.ThreadGateway,
	runs agent.RunStore,
	o11y observability.Observability,
) *BudgetCreationContinuer {
	total := o11y.Metrics().Counter(
		"agents_budget_creation_total",
		"Total de execucoes do fluxo de criacao conversacional de orcamento",
		"1",
	)
	return &BudgetCreationContinuer{
		engine:  engine,
		def:     def,
		threads: threads,
		runs:    runs,
		o11y:    o11y,
		total:   total,
	}
}

func (c *BudgetCreationContinuer) Continue(ctx context.Context, resourceID, text, messageID string) (bool, string, error) {
	ctx, span := c.o11y.Tracer().Start(ctx, "agents.usecase.budget_creation_continuer",
		observability.WithAttributes(
			observability.String("wamid", messageID),
		),
	)
	defer span.End()

	start := time.Now()

	run, runErr := c.openRun(ctx, resourceID, messageID, start)
	if runErr != nil {
		span.RecordError(runErr)
		c.total.Add(ctx, 1, observability.String("outcome", "error"))
		return false, "", fmt.Errorf("agents.usecase.budget_creation_continuer: open_run: %w", runErr)
	}

	key := workflows.BudgetCreationKey(resourceID)

	patch, err := json.Marshal(map[string]string{
		"resumeText":        text,
		"incomingMessageId": messageID,
	})
	if err != nil {
		c.recordFailure(ctx, span, run, start, "marshal patch", err)
		return false, "", fmt.Errorf("agents.usecase.budget_creation_continuer: marshal patch: %w", err)
	}

	result, err := c.engine.Resume(ctx, c.def, key, patch)
	if err != nil {
		if result.Status == workflow.RunStatusFailed && result.State.ResponseText != "" {
			c.recordFailure(ctx, span, run, start, "step_failed", err)
			return true, result.State.ResponseText, nil
		}
		c.recordFailure(ctx, span, run, start, "resume", err)
		return false, "", fmt.Errorf("agents.usecase.budget_creation_continuer: resume: %w", err)
	}

	if result.Status == 0 {
		c.closeRun(ctx, run, agent.RunStatusSucceeded, "", start)
		return false, "", nil
	}

	c.closeRun(ctx, run, agent.RunStatusSucceeded, "", start)

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

	c.total.Add(ctx, 1, observability.String("outcome", budgetCreationOutcomeString(result.State.Status)))
	return true, result.State.ResponseText, nil
}

func budgetCreationOutcomeString(status workflows.BudgetCreationStatus) string {
	switch status {
	case workflows.BudgetCreationCompleted:
		return "completed"
	case workflows.BudgetCreationCancelled:
		return "cancelled"
	case workflows.BudgetCreationExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (c *BudgetCreationContinuer) openRun(ctx context.Context, resourceID, wamid string, start time.Time) (agent.Run, error) {
	thread, err := c.threads.GetOrCreate(ctx, resourceID, resourceID)
	if err != nil {
		return agent.Run{}, fmt.Errorf("get_or_create_thread: %w", err)
	}

	run := agent.Run{
		ID:               uuid.New(),
		PlatformThreadID: thread.ID,
		ResourceID:       resourceID,
		ThreadID:         resourceID,
		AgentID:          mecontrolaAgentID,
		Workflow:         workflows.BudgetCreationWorkflowID,
		CorrelationKey:   wamid,
		Status:           agent.RunStatusRunning,
		StartedAt:        start.UTC(),
	}
	if err := c.runs.Insert(ctx, run); err != nil {
		return agent.Run{}, fmt.Errorf("insert_run: %w", err)
	}
	return run, nil
}

func (c *BudgetCreationContinuer) recordFailure(ctx context.Context, span observability.Span, run agent.Run, start time.Time, stage string, err error) {
	span.RecordError(err)
	span.SetStatus(observability.StatusCodeError, stage)
	c.total.Add(ctx, 1, observability.String("outcome", "error"))
	c.closeRun(ctx, run, agent.RunStatusFailed, err.Error(), start)
	c.o11y.Logger().Error(ctx, "agents.usecase.budget_creation_continuer: "+stage+" falhou",
		observability.String("thread_id", run.ThreadID),
		observability.String("run_id", run.ID.String()),
		observability.String("wamid", run.CorrelationKey),
		observability.Error(err),
	)
}

func (c *BudgetCreationContinuer) closeRun(ctx context.Context, run agent.Run, status agent.RunStatus, errStr string, start time.Time) {
	if run.ID == uuid.Nil {
		return
	}
	now := time.Now().UTC()
	run.Status = status
	run.Error = errStr
	run.EndedAt = &now
	run.DurationMs = time.Since(start).Milliseconds()
	_ = c.runs.Update(ctx, run)
}
