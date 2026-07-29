package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

const (
	dedupHousekeepingDefaultRetentionDays = 30
	dedupHousekeepingDefaultBatchSize     = 10_000
)

type CleanupProcessedMessages struct {
	repo         MessageRepository
	consumerRepo ConsumerMessageRepository
	cfg          configs.WhatsAppConfig
	o11y         observability.Observability
	deletedTotal observability.Counter
	duration     observability.Histogram
}

type CleanupOption func(*CleanupProcessedMessages)

func WithConsumerRepository(repo ConsumerMessageRepository) CleanupOption {
	return func(u *CleanupProcessedMessages) {
		u.consumerRepo = repo
	}
}

func NewCleanupProcessedMessages(
	repo MessageRepository,
	cfg configs.WhatsAppConfig,
	o11y observability.Observability,
	opts ...CleanupOption,
) *CleanupProcessedMessages {
	deletedTotal := o11y.Metrics().Counter(
		"whatsapp_dedup_housekeeping_deleted_total",
		"Total de linhas de channel_processed_messages apagadas pelo housekeeping",
		"1",
	)
	duration := o11y.Metrics().Histogram(
		"whatsapp_dedup_housekeeping_duration_seconds",
		"Duração total de cada execução do housekeeping de dedup do WhatsApp",
		"s",
	)
	u := &CleanupProcessedMessages{
		repo:         repo,
		cfg:          cfg,
		o11y:         o11y,
		deletedTotal: deletedTotal,
		duration:     duration,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *CleanupProcessedMessages) Execute(ctx context.Context) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "whatsapp.dedup.usecase.cleanup_processed_messages")
	defer span.End()

	start := time.Now()

	retentionDays := u.cfg.DedupHousekeepingRetentionDays
	if retentionDays <= 0 {
		retentionDays = dedupHousekeepingDefaultRetentionDays
	}

	batchSize := u.cfg.DedupHousekeepingBatch
	if batchSize <= 0 {
		batchSize = dedupHousekeepingDefaultBatchSize
	}

	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	var total int64
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("whatsapp.dedup.usecase.cleanup_processed_messages: context cancelled: %w", ctx.Err())
		default:
		}

		n, err := u.repo.DeleteProcessedBefore(ctx, cutoff, batchSize)
		if err != nil {
			span.RecordError(err)
			u.o11y.Logger().Error(ctx, "whatsapp.dedup.housekeeping.delete_failed",
				observability.Error(err),
			)
			return fmt.Errorf("whatsapp.dedup.usecase.cleanup_processed_messages delete_processed_before: %w", err)
		}

		total += n
		if n == 0 {
			break
		}
	}

	if u.consumerRepo != nil {
		n, err := u.consumerRepo.DeleteProcessedBefore(ctx, cutoff)
		if err != nil {
			span.RecordError(err)
			u.o11y.Logger().Error(ctx, "whatsapp.dedup.housekeeping.consumer_delete_failed",
				observability.Error(err),
			)
			return fmt.Errorf("whatsapp.dedup.usecase.cleanup_processed_messages consumer_delete_processed_before: %w", err)
		}
		total += n
	}

	elapsed := time.Since(start).Seconds()
	u.duration.Record(ctx, elapsed)

	if total > 0 {
		u.deletedTotal.Add(ctx, total)
		u.o11y.Logger().Info(ctx, "whatsapp.dedup.housekeeping.deleted",
			observability.Int64("count", total),
			observability.Int("retention_days", retentionDays),
		)
	}

	return nil
}
