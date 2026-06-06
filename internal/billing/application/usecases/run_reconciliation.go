package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

const reconciliationCheckpointName = "kiwify_sales"
const reconciliationWindowOverlap = 15 * time.Minute
const reconciliationDefaultLookback = time.Hour

type RunReconciliation struct {
	db        database.DBTX
	factory   interfaces.RepositoryFactory
	reconcile *ReconcileSubscriptions
	o11y      observability.Observability
}

func NewRunReconciliation(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	reconcile *ReconcileSubscriptions,
	o11y observability.Observability,
) *RunReconciliation {
	return &RunReconciliation{db: db, factory: factory, reconcile: reconcile, o11y: o11y}
}

func (u *RunReconciliation) Execute(ctx context.Context) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "billing.usecase.run_reconciliation")
	defer span.End()

	startedAt := time.Now().UTC()
	checkpointRepo := u.factory.ReconciliationCheckpointRepository(u.db)
	checkpoint, err := checkpointRepo.Get(ctx, reconciliationCheckpointName)
	if errors.Is(err, application.ErrCheckpointNotFound) {
		checkpoint = time.Now().UTC().Add(-reconciliationDefaultLookback)
		u.o11y.Logger().Info(ctx, "billing.reconciliation.run.checkpoint_missing_using_default",
			observability.String("default_lookback", reconciliationDefaultLookback.String()),
		)
	}
	if err != nil && !errors.Is(err, application.ErrCheckpointNotFound) {
		return fmt.Errorf("billing.usecase.run_reconciliation get checkpoint: %w", err)
	}

	windowStart := checkpoint.Add(-reconciliationWindowOverlap)
	windowEnd := time.Now().UTC()
	if err := u.reconcile.Execute(ctx, input.ReconcileSubscriptionsInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}); err != nil {
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "billing.reconciliation.run.failed",
			observability.String("window_start", windowStart.Format(time.RFC3339)),
			observability.String("window_end", windowEnd.Format(time.RFC3339)),
			observability.Error(err),
		)
		return fmt.Errorf("billing.usecase.run_reconciliation execute: %w", err)
	}

	u.o11y.Logger().Info(ctx, "billing.reconciliation.run",
		observability.String("window_start", windowStart.Format(time.RFC3339)),
		observability.String("window_end", windowEnd.Format(time.RFC3339)),
		observability.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	return nil
}
