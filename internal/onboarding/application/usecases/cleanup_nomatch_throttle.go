package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

const (
	nomatchThrottleDefaultRetentionDays = 30
	nomatchThrottleDefaultBatchSize     = 10_000
)

type CleanupNomatchThrottle struct {
	repo         interfaces.NoMatchThrottle
	cfg          configs.OnboardingConfig
	o11y         observability.Observability
	deletedTotal observability.Counter
	duration     observability.Histogram
}

func NewCleanupNomatchThrottle(
	repo interfaces.NoMatchThrottle,
	cfg configs.OnboardingConfig,
	o11y observability.Observability,
) *CleanupNomatchThrottle {
	deletedTotal := o11y.Metrics().Counter(
		"onboarding_activation_nomatch_throttle_housekeeping_deleted_total",
		"Total de linhas de nomatch_throttle apagadas pelo housekeeping",
		"1",
	)
	duration := o11y.Metrics().Histogram(
		"onboarding_activation_nomatch_throttle_housekeeping_duration_seconds",
		"Duracao total de cada execucao do housekeeping de nomatch throttle",
		"s",
	)
	return &CleanupNomatchThrottle{
		repo:         repo,
		cfg:          cfg,
		o11y:         o11y,
		deletedTotal: deletedTotal,
		duration:     duration,
	}
}

func (u *CleanupNomatchThrottle) Execute(ctx context.Context) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "onboarding.usecase.cleanup_nomatch_throttle")
	defer span.End()

	start := time.Now()

	retentionDays := u.cfg.ActivationNoMatchThrottleRetentionDays
	if retentionDays <= 0 {
		retentionDays = nomatchThrottleDefaultRetentionDays
	}

	batchSize := u.cfg.ActivationNoMatchThrottleBatch
	if batchSize <= 0 {
		batchSize = nomatchThrottleDefaultBatchSize
	}

	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	var total int64
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("onboarding: cleanup_nomatch_throttle: context cancelled: %w", ctx.Err())
		default:
		}

		n, err := u.repo.DeleteBefore(ctx, cutoff, batchSize)
		if err != nil {
			span.RecordError(err)
			u.o11y.Logger().Error(ctx, "onboarding.nomatch_throttle.housekeeping.delete_failed",
				observability.Error(err),
			)
			return fmt.Errorf("onboarding: cleanup_nomatch_throttle: delete_before: %w", err)
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
		u.o11y.Logger().Info(ctx, "onboarding.nomatch_throttle.housekeeping.deleted",
			observability.Int64("count", total),
			observability.Int("retention_days", retentionDays),
		)
	}

	return nil
}
