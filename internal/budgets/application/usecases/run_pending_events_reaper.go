package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

const _pendingReaperBatchSize = 200

type RunPendingEventsReaper struct {
	pending   interfaces.PendingEventRepository
	apply     *ApplyPendingEvent
	uow       uow.UnitOfWork[struct{}]
	o11y      observability.Observability
	processed observability.Counter
	expired   observability.Counter
	applied   observability.Counter
}

func NewRunPendingEventsReaper(
	pending interfaces.PendingEventRepository,
	apply *ApplyPendingEvent,
	u uow.UnitOfWork[struct{}],
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
		pending:   pending,
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

	_, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		events, listErr := uc.pending.ListReady(ctx, tx, _pendingReaperBatchSize)
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
	outcome, applyErr := uc.apply.Execute(ctx, tx, evt)
	if applyErr != nil {
		return fmt.Errorf("budgets.usecase.run_pending_events_reaper: aplicar evento: %w", applyErr)
	}

	switch outcome {
	case PendingEventOutcomeApplied:
		if transErr := uc.pending.Transition(ctx, tx, evt.ID(), entities.PendingStateApplied, "applied"); transErr != nil {
			return fmt.Errorf("budgets.usecase.run_pending_events_reaper: transitar applied: %w", transErr)
		}
		uc.applied.Add(ctx, 1)

	case PendingEventOutcomeExpired:
		if transErr := uc.pending.Transition(ctx, tx, evt.ID(), entities.PendingStateExpired, "ttl_exceeded"); transErr != nil {
			return fmt.Errorf("budgets.usecase.run_pending_events_reaper: transitar expired: %w", transErr)
		}
		uc.expired.Add(ctx, 1)

	case PendingEventOutcomeObsoleteIdempotent:
		if transErr := uc.pending.Transition(ctx, tx, evt.ID(), entities.PendingStateApplied, "obsolete_idempotent"); transErr != nil {
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
