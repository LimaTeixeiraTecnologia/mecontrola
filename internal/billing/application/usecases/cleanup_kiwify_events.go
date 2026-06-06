package usecases

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

type CleanupKiwifyEvents struct {
	db      database.DBTX
	factory interfaces.RepositoryFactory
	cfg     configs.BillingConfig
	o11y    observability.Observability
}

func NewCleanupKiwifyEvents(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	cfg configs.BillingConfig,
	o11y observability.Observability,
) *CleanupKiwifyEvents {
	return &CleanupKiwifyEvents{db: db, factory: factory, cfg: cfg, o11y: o11y}
}

func (u *CleanupKiwifyEvents) Execute(ctx context.Context) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "billing.usecase.cleanup_kiwify_events")
	defer span.End()

	retentionDays := u.cfg.KiwifyEventsRetentionDays
	if retentionDays <= 0 {
		retentionDays = housekeepingDefaultRetentionDays
	}

	batchSize := u.cfg.KiwifyEventsHousekeepingBatch
	if batchSize <= 0 {
		batchSize = housekeepingDefaultBatch
	}

	before := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	repo := u.factory.KiwifyEventRepository(u.db)

	var total int64
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("billing.usecase.cleanup_kiwify_events: context cancelled: %w", ctx.Err())
		default:
		}

		n, err := repo.DeleteOlderThan(ctx, before, batchSize)
		if err != nil {
			span.RecordError(err)
			u.o11y.Logger().Error(ctx, "billing.kiwify_events_housekeeping.delete_failed",
				observability.Error(err),
			)
			return fmt.Errorf("billing.usecase.cleanup_kiwify_events delete older than: %w", err)
		}

		total += n
		if n == 0 {
			break
		}
	}

	if total > 0 {
		u.o11y.Logger().Info(ctx, "billing.kiwify_events_housekeeping.deleted",
			observability.Int64("count", total),
			observability.Int("retention_days", retentionDays),
		)
	}

	return nil
}
