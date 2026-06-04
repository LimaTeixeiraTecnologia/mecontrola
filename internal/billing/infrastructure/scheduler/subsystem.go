package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	robfigcron "github.com/robfig/cron/v3"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
)

// Deps agrupa as dependências do BillingScheduler.
type Deps struct {
	ReconcileUseCase  *usecases.ReconcileSubscriptionsUseCase
	AnonymizeUseCase  *usecases.AnonymizeWebhookEventsUseCase
	ReconcileSchedule string
	AnonymizeSchedule string
	Logger            *slog.Logger
}

// BillingScheduler é o runner que agenda os jobs periódicos de billing (ADR-003).
// Usa robfig/cron/v3 já presente em go.mod. Cada job utiliza semáforo TryLock para
// evitar execuções sobrepostas.
type BillingScheduler struct {
	cron              *robfigcron.Cron
	reconcileJob      *reconciliationJob
	anonymizeJob      *anonymizationJob
	reconcileSchedule string
	anonymizeSchedule string
	logger            *slog.Logger
}

// NewBillingScheduler cria um BillingScheduler com os jobs de reconciliação e anonimização.
func NewBillingScheduler(deps Deps) *BillingScheduler {
	return &BillingScheduler{
		cron:              robfigcron.New(robfigcron.WithSeconds()),
		reconcileJob:      newReconciliationJob(deps.ReconcileUseCase, deps.Logger),
		anonymizeJob:      newAnonymizationJob(deps.AnonymizeUseCase, deps.Logger),
		reconcileSchedule: deps.ReconcileSchedule,
		anonymizeSchedule: deps.AnonymizeSchedule,
		logger:            deps.Logger,
	}
}

// Name retorna o identificador do runner.
func (s *BillingScheduler) Name() string { return "billing-scheduler" }

// Start registra os jobs no cron e inicia o agendador.
func (s *BillingScheduler) Start(ctx context.Context) error {
	if _, err := s.cron.AddFunc(s.reconcileSchedule, s.reconcileJob.run); err != nil {
		return fmt.Errorf("billing scheduler: agendamento reconciliação: %w", err)
	}
	s.logger.InfoContext(ctx, "billing reconciliation agendada", "schedule", s.reconcileSchedule)

	if _, err := s.cron.AddFunc(s.anonymizeSchedule, s.anonymizeJob.run); err != nil {
		return fmt.Errorf("billing scheduler: agendamento anonimização: %w", err)
	}
	s.logger.InfoContext(ctx, "billing anonymization agendada", "schedule", s.anonymizeSchedule)

	s.cron.Start()
	return nil
}

// Stop para o agendador aguardando os jobs em execução terminarem.
func (s *BillingScheduler) Stop(ctx context.Context) error {
	if s.cron == nil {
		return nil
	}
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
