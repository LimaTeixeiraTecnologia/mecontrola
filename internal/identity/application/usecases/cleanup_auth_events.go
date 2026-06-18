package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

const (
	authEventsDefaultRetentionDays = 180
	authEventsDefaultBatchSize     = 10_000
)

type CleanupAuthEvents struct {
	repo         interfaces.AuthEventsRepository
	cfg          configs.IdentityConfig
	o11y         observability.Observability
	deletedTotal observability.Counter
	duration     observability.Histogram
}

func NewCleanupAuthEvents(
	repo interfaces.AuthEventsRepository,
	cfg configs.IdentityConfig,
	o11y observability.Observability,
) *CleanupAuthEvents {
	deletedTotal := o11y.Metrics().Counter(
		"auth_events_housekeeping_deleted_total",
		"Total de linhas de auth_events apagadas pelo housekeeping",
		"1",
	)
	duration := o11y.Metrics().Histogram(
		"auth_events_housekeeping_duration_seconds",
		"Duração total de cada execução do housekeeping de auth_events",
		"s",
	)
	return &CleanupAuthEvents{
		repo:         repo,
		cfg:          cfg,
		o11y:         o11y,
		deletedTotal: deletedTotal,
		duration:     duration,
	}
}

func (u *CleanupAuthEvents) Execute(ctx context.Context) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.cleanup_auth_events")
	defer span.End()

	start := time.Now()

	retentionDays := u.cfg.AuthEventsRetentionDays
	if retentionDays <= 0 {
		retentionDays = authEventsDefaultRetentionDays
	}

	batchSize := u.cfg.AuthEventsHousekeepingBatch
	if batchSize <= 0 {
		batchSize = authEventsDefaultBatchSize
	}

	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	var total int64
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("identity.usecase.cleanup_auth_events: context cancelled: %w", ctx.Err())
		default:
		}

		n, err := u.repo.DeleteOlderThan(ctx, cutoff, batchSize)
		if err != nil {
			span.RecordError(err)
			u.o11y.Logger().Error(ctx, "identity.auth_events_housekeeping.delete_failed",
				observability.Error(err),
			)
			return fmt.Errorf("identity.usecase.cleanup_auth_events delete_older_than: %w", err)
		}

		total += n
		if n == 0 {
			break
		}
	}

	elapsed := time.Since(start).Seconds()
	u.duration.Record(ctx, elapsed)

	if total > 0 {
		u.deletedTotal.Add(ctx, total)
		u.o11y.Logger().Info(ctx, "identity.auth_events_housekeeping.deleted",
			observability.Int64("count", total),
			observability.Int("retention_days", retentionDays),
		)
	}

	return nil
}
