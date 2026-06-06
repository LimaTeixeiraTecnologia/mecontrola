package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
)

const reconciliationCheckpointName = "kiwify_sales"
const reconciliationWindowOverlap = 15 * time.Minute
const reconciliationDefaultLookback = time.Hour

type ReconciliationJob struct {
	db          database.DBTX
	factory     interfaces.RepositoryFactory
	reconcile   *usecases.ReconcileSubscriptions
	cfg         configs.KiwifyConfig
	o11y        observability.Observability
	corrections observability.Counter
}

func NewReconciliationJob(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	reconcile *usecases.ReconcileSubscriptions,
	cfg configs.KiwifyConfig,
	o11y observability.Observability,
) *ReconciliationJob {
	corrections := o11y.Metrics().Counter(
		"billing_reconciliation_corrections_total",
		"Total de correções aplicadas durante reconciliação",
		"1",
	)
	return &ReconciliationJob{
		db:          db,
		factory:     factory,
		reconcile:   reconcile,
		cfg:         cfg,
		o11y:        o11y,
		corrections: corrections,
	}
}

func (j *ReconciliationJob) Name() string     { return "billing-reconciliation" }
func (j *ReconciliationJob) Schedule() string { return j.cfg.ReconciliationInterval }

func (j *ReconciliationJob) Run(ctx context.Context) error {
	ctx, span := j.o11y.Tracer().Start(ctx, "billing.job.reconciliation.run")
	defer span.End()

	start := time.Now().UTC()

	checkpointRepo := j.factory.ReconciliationCheckpointRepository(j.db)
	checkpoint, err := checkpointRepo.Get(ctx, reconciliationCheckpointName)
	if err != nil {
		checkpoint = time.Now().UTC().Add(-reconciliationDefaultLookback)
		j.o11y.Logger().Info(ctx, "billing.reconciliation.run.checkpoint_missing_using_default",
			observability.String("default_lookback", reconciliationDefaultLookback.String()),
		)
	}

	windowStart := checkpoint.Add(-reconciliationWindowOverlap)
	windowEnd := time.Now().UTC()

	if err := j.reconcile.Execute(ctx, input.ReconcileSubscriptionsInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}); err != nil {
		span.RecordError(err)
		j.o11y.Logger().Error(ctx, "billing.reconciliation.run.failed",
			observability.String("window_start", windowStart.Format(time.RFC3339)),
			observability.String("window_end", windowEnd.Format(time.RFC3339)),
			observability.Error(err),
		)
		return fmt.Errorf("billing.job.reconciliation: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()
	j.o11y.Logger().Info(ctx, "billing.reconciliation.run",
		observability.String("window_start", windowStart.Format(time.RFC3339)),
		observability.String("window_end", windowEnd.Format(time.RFC3339)),
		observability.Int64("duration_ms", durationMs),
	)

	return nil
}
