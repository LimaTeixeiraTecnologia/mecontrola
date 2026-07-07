package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PendingEntryContinuer struct {
	engine       workflow.Engine[workflows.PendingEntryState]
	def          workflow.Definition[workflows.PendingEntryState]
	o11y         observability.Observability
	total        observability.Counter
	slotTotal    observability.Counter
	writeTotal   observability.Counter
	durationHist observability.Histogram
}

func NewPendingEntryContinuer(
	engine workflow.Engine[workflows.PendingEntryState],
	def workflow.Definition[workflows.PendingEntryState],
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
		engine:       engine,
		def:          def,
		o11y:         o11y,
		total:        total,
		slotTotal:    slotTotal,
		writeTotal:   writeTotal,
		durationHist: durationHist,
	}
}

func (c *PendingEntryContinuer) Continue(ctx context.Context, userID, peer, message, messageID string) (workflows.PendingEntryResult, error) {
	ctx, span := c.o11y.Tracer().Start(ctx, "agents.usecase.pending_entry_continuer")
	defer span.End()

	start := time.Now()

	key := workflows.PendingEntryKey(userID, peer)
	patch, err := json.Marshal(map[string]string{
		"resumeText":        message,
		"incomingMessageId": messageID,
	})
	if err != nil {
		span.RecordError(err)
		c.total.Add(ctx, 1, observability.String("outcome", "error"))
		c.durationHist.Record(ctx, time.Since(start).Seconds(), observability.String("outcome", "error"))
		return workflows.PendingEntryResult{}, fmt.Errorf("agents.usecase.pending_entry_continuer: marshal patch: %w", err)
	}

	result, err := c.engine.Resume(ctx, c.def, key, patch)
	if err != nil {
		span.RecordError(err)
		c.total.Add(ctx, 1, observability.String("outcome", "error"))
		c.durationHist.Record(ctx, time.Since(start).Seconds(), observability.String("outcome", "error"))
		return workflows.PendingEntryResult{}, fmt.Errorf("agents.usecase.pending_entry_continuer: resume: %w", err)
	}

	if result.Status == 0 {
		return workflows.PendingEntryResult{Handled: false}, nil
	}

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
