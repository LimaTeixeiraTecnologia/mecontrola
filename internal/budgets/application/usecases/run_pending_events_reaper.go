package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

const _pendingReaperBatchSize = 200

type RunPendingEventsReaper struct {
	factory   interfaces.RepositoryFactory
	apply     *ApplyPendingEvent
	uow       uow.UnitOfWork
	o11y      observability.Observability
	processed observability.Counter
	expired   observability.Counter
	applied   observability.Counter
}

func NewRunPendingEventsReaper(
	factory interfaces.RepositoryFactory,
	apply *ApplyPendingEvent,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *RunPendingEventsReaper {
	processed := o11y.Metrics().Counter(
		"budgets_pending_events_total",
		"Total de eventos pendentes processados pelo reaper",
		"1",
	)
	exp := o11y.Metrics().Counter(
		"budgets_pending_events_expired_total",
		"Total de eventos pendentes expirados pelo reaper",
		"1",
	)
	app := o11y.Metrics().Counter(
		"budgets_pending_events_applied_total",
		"Total de eventos pendentes aplicados pelo reaper",
		"1",
	)
	return &RunPendingEventsReaper{
		factory:   factory,
		apply:     apply,
		uow:       u,
		o11y:      o11y,
		processed: processed,
		expired:   exp,
		applied:   app,
	}
}

func (uc *RunPendingEventsReaper) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.run_pending_events_reaper")
	defer span.End()

	_, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		pending := uc.factory.PendingEventRepository(tx)
		events, listErr := pending.ListReady(ctx, _pendingReaperBatchSize)
		if listErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.run_pending_events_reaper: listar prontos: %w", listErr)
		}

		if len(events) == 0 {
			return struct{}{}, nil
		}

		uc.recordOldest(ctx, events)

		for _, evt := range events {
			if err := uc.processOne(ctx, tx, evt); err != nil {
				uc.o11y.Logger().Warn(ctx, "budgets.usecase.run_pending_events_reaper.process_failed",
					observability.String("event_id", evt.EventID().String()),
					observability.Error(err),
				)
			}
			uc.processed.Add(ctx, 1)
		}

		return struct{}{}, nil
	})

	if err != nil {
		span.RecordError(err)
	}
	return err
}

func (uc *RunPendingEventsReaper) processOne(ctx context.Context, tx database.DBTX, evt entities.PendingEvent) error {
	pending := uc.factory.PendingEventRepository(tx)
	outcome, applyErr := uc.apply.Execute(ctx, tx, evt)
	if applyErr != nil {
		return fmt.Errorf("budgets.usecase.run_pending_events_reaper: aplicar evento: %w", applyErr)
	}

	switch outcome {
	case PendingEventOutcomeApplied:
		if transErr := pending.Transition(ctx, evt.ID(), evt.UserID(), entities.PendingStateApplied, "applied"); transErr != nil {
			return fmt.Errorf("budgets.usecase.run_pending_events_reaper: transitar applied: %w", transErr)
		}
		uc.applied.Add(ctx, 1)

	case PendingEventOutcomeExpired:
		if transErr := pending.Transition(ctx, evt.ID(), evt.UserID(), entities.PendingStateExpired, "ttl_exceeded"); transErr != nil {
			return fmt.Errorf("budgets.usecase.run_pending_events_reaper: transitar expired: %w", transErr)
		}
		uc.expired.Add(ctx, 1)

	case PendingEventOutcomeObsoleteIdempotent:
		if transErr := pending.Transition(ctx, evt.ID(), evt.UserID(), entities.PendingStateApplied, "obsolete_idempotent"); transErr != nil {
			return fmt.Errorf("budgets.usecase.run_pending_events_reaper: transitar obsolete: %w", transErr)
		}

	case PendingEventOutcomeStillPending:
	}

	return nil
}

func (uc *RunPendingEventsReaper) recordOldest(ctx context.Context, events []entities.PendingEvent) {
	if len(events) == 0 {
		return
	}
	oldest := events[0].ReceivedAt()
	for _, e := range events[1:] {
		if e.ReceivedAt().Before(oldest) {
			oldest = e.ReceivedAt()
		}
	}
	age := time.Since(oldest).Seconds()
	uc.o11y.Logger().Info(ctx, "budgets.usecase.run_pending_events_reaper.oldest_age_seconds",
		observability.Float64("age_seconds", age),
	)
}
