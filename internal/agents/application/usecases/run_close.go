package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const runUpdateErrorsMetric = "agents_run_update_errors_total"

func newRunUpdateErrorsCounter(o11y observability.Observability) observability.Counter {
	return o11y.Metrics().Counter(
		runUpdateErrorsMetric,
		"Total de falhas ao fechar (RunStore.Update) um run auditavel de retomada de pendencia",
		"1",
	)
}

func closeObservedRun(
	ctx context.Context,
	runs agent.RunStore,
	o11y observability.Observability,
	runUpdateErrors observability.Counter,
	run agent.Run,
	status agent.RunStatus,
	stage, errStr string,
	start time.Time,
) {
	if run.ID == uuid.Nil {
		return
	}
	now := time.Now().UTC()
	run.Status = status
	run.Error = errStr
	run.EndedAt = &now
	run.DurationMs = time.Since(start).Milliseconds()

	if err := runs.Update(ctx, run); err != nil {
		runUpdateErrors.Increment(ctx,
			observability.String("workflow", run.Workflow),
			observability.String("stage", stage),
			observability.String("status", status.String()),
		)
		o11y.Logger().Error(ctx, "agents.usecase.continuer.close_run: run_store.update falhou",
			observability.String("run_id", run.ID.String()),
			observability.String("wamid", run.CorrelationKey),
			observability.String("workflow", run.Workflow),
			observability.String("stage", stage),
			observability.String("status", status.String()),
			observability.Error(err),
		)
	}
}
