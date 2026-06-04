package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
)

const reconciliationJobTimeout = 50 * time.Minute

// reconciliationJob executa ReconcileSubscriptionsUseCase com semáforo TryLock
// para evitar sobreposição de execuções (ADR-003).
type reconciliationJob struct {
	useCase *usecases.ReconcileSubscriptionsUseCase
	logger  *slog.Logger
	mu      sync.Mutex
}

func newReconciliationJob(useCase *usecases.ReconcileSubscriptionsUseCase, logger *slog.Logger) *reconciliationJob {
	return &reconciliationJob{useCase: useCase, logger: logger}
}

func (j *reconciliationJob) run() {
	if j.useCase == nil {
		return
	}
	if !j.mu.TryLock() {
		j.logger.Warn("billing reconciliation: execução anterior ainda em andamento, pulando tick")
		return
	}
	defer j.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), reconciliationJobTimeout)
	defer cancel()

	report, err := j.useCase.Execute(ctx)
	if err != nil {
		j.logger.ErrorContext(ctx, "billing reconciliation: erro", "error", err)
		return
	}
	j.logger.InfoContext(ctx, "billing reconciliation: concluída",
		"inspected", report.Inspected,
		"diverged", report.Diverged,
		"synced", report.Synced,
	)
}
