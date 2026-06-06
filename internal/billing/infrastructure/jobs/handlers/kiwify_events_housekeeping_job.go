package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

const housekeepingDefaultRetentionDays = 90
const housekeepingDefaultBatch = 500

type KiwifyEventsHousekeepingJob struct {
	db      database.DBTX
	factory interfaces.RepositoryFactory
	cfg     configs.BillingConfig
	o11y    observability.Observability
}

func NewKiwifyEventsHousekeepingJob(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	cfg configs.BillingConfig,
	o11y observability.Observability,
) *KiwifyEventsHousekeepingJob {
	return &KiwifyEventsHousekeepingJob{
		db:      db,
		factory: factory,
		cfg:     cfg,
		o11y:    o11y,
	}
}

func (j *KiwifyEventsHousekeepingJob) Name() string { return "billing-kiwify-events-housekeeping" }
func (j *KiwifyEventsHousekeepingJob) Schedule() string {
	if j.cfg.KiwifyEventsHousekeepingSchedule != "" {
		return j.cfg.KiwifyEventsHousekeepingSchedule
	}
	return "@daily"
}

func (j *KiwifyEventsHousekeepingJob) Run(ctx context.Context) error {
	ctx, span := j.o11y.Tracer().Start(ctx, "billing.job.kiwify_events_housekeeping.run")
	defer span.End()

	retentionDays := j.cfg.KiwifyEventsRetentionDays
	if retentionDays <= 0 {
		retentionDays = housekeepingDefaultRetentionDays
	}

	batchSize := j.cfg.KiwifyEventsHousekeepingBatch
	if batchSize <= 0 {
		batchSize = housekeepingDefaultBatch
	}

	before := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	repo := j.factory.KiwifyEventRepository(j.db)

	var total int64
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("billing.job.kiwify_events_housekeeping: context cancelled: %w", ctx.Err())
		default:
		}

		n, err := repo.DeleteOlderThan(ctx, before, batchSize)
		if err != nil {
			span.RecordError(err)
			j.o11y.Logger().Error(ctx, "billing.kiwify_events_housekeeping.delete_failed",
				observability.Error(err),
			)
			return fmt.Errorf("billing.job.kiwify_events_housekeeping: %w", err)
		}

		total += n
		if n == 0 {
			break
		}
	}

	if total > 0 {
		j.o11y.Logger().Info(ctx, "billing.kiwify_events_housekeeping.deleted",
			observability.Int64("count", total),
			observability.Int("retention_days", retentionDays),
		)
	}

	return nil
}
