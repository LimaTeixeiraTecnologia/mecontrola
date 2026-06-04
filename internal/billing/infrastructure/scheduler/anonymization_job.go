package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
)

const (
	anonymizationJobTimeout    = 55 * time.Minute
	anonymizationRetentionDays = 365
	anonymizationBatchSize     = 500
)

// anonymizationJob executa AnonymizeWebhookEventsUseCase com semáforo TryLock
// para evitar sobreposição de execuções (ADR-003).
type anonymizationJob struct {
	useCase       *usecases.AnonymizeWebhookEventsUseCase
	logger        *slog.Logger
	mu            sync.Mutex
	retentionDays int
	batchSize     int
}

func newAnonymizationJob(useCase *usecases.AnonymizeWebhookEventsUseCase, logger *slog.Logger) *anonymizationJob {
	return &anonymizationJob{
		useCase:       useCase,
		logger:        logger,
		retentionDays: anonymizationRetentionDays,
		batchSize:     anonymizationBatchSize,
	}
}

func (j *anonymizationJob) run() {
	if j.useCase == nil {
		return
	}
	if !j.mu.TryLock() {
		j.logger.Warn("billing anonymization: execução anterior ainda em andamento, pulando tick")
		return
	}
	defer j.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), anonymizationJobTimeout)
	defer cancel()

	olderThan := time.Now().AddDate(0, 0, -j.retentionDays)
	report, err := j.useCase.Execute(ctx, input.AnonymizeInput{
		OlderThan: olderThan,
		BatchSize: j.batchSize,
	})
	if err != nil {
		j.logger.ErrorContext(ctx, "billing anonymization: erro", "error", err)
		return
	}
	j.logger.InfoContext(ctx, "billing anonymization: concluída",
		"processed", report.Processed,
		"errors", report.Errors,
	)
}
