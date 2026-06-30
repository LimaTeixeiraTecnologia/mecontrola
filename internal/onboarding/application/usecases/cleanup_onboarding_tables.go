package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

const cleanupBatchSize = 1000

type CleanupOnboardingTables struct {
	repo            appinterfaces.OnboardingCleanupRepository
	retentionPeriod time.Duration
	o11y            observability.Observability
}

func NewCleanupOnboardingTables(
	repo appinterfaces.OnboardingCleanupRepository,
	retentionPeriod time.Duration,
	o11y observability.Observability,
) *CleanupOnboardingTables {
	return &CleanupOnboardingTables{
		repo:            repo,
		retentionPeriod: retentionPeriod,
		o11y:            o11y,
	}
}

func (uc *CleanupOnboardingTables) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.cleanup_onboarding_tables")
	defer span.End()

	before := time.Now().UTC().Add(-uc.retentionPeriod)

	if err := uc.deleteMetaProcessed(ctx, before); err != nil {
		span.RecordError(err)
		return err
	}
	if err := uc.deleteConsumerLookupAttempts(ctx, before); err != nil {
		span.RecordError(err)
		return err
	}
	return uc.deleteWelcomeProcessed(ctx, before)
}

func (uc *CleanupOnboardingTables) deleteWelcomeProcessed(
	ctx context.Context,
	before time.Time,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("onboarding: cleanup onboarding_welcome_processed: context cancelled: %w", err)
		}
		deleted, err := uc.repo.DeleteWelcomeProcessedOlderThan(ctx, before, cleanupBatchSize)
		if err != nil {
			return fmt.Errorf("onboarding: cleanup onboarding_welcome_processed: %w", err)
		}
		if deleted < int64(cleanupBatchSize) {
			break
		}
	}
	return nil
}

func (uc *CleanupOnboardingTables) deleteMetaProcessed(
	ctx context.Context,
	before time.Time,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("onboarding: cleanup channel_processed_messages: context cancelled: %w", err)
		}
		deleted, err := uc.repo.DeleteMetaProcessedOlderThan(ctx, before, cleanupBatchSize)
		if err != nil {
			return fmt.Errorf("onboarding: cleanup channel_processed_messages: %w", err)
		}
		if deleted < int64(cleanupBatchSize) {
			break
		}
	}
	return nil
}

func (uc *CleanupOnboardingTables) deleteConsumerLookupAttempts(
	ctx context.Context,
	before time.Time,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("onboarding: cleanup consumer_lookup_attempts: context cancelled: %w", err)
		}
		deleted, err := uc.repo.DeleteConsumerLookupAttemptsOlderThan(ctx, before, cleanupBatchSize)
		if err != nil {
			return fmt.Errorf("onboarding: cleanup consumer_lookup_attempts: %w", err)
		}
		if deleted < int64(cleanupBatchSize) {
			break
		}
	}
	return nil
}
