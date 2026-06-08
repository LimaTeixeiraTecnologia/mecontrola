package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

const cleanupBatchSize = 1000

type CleanupOnboardingTables struct {
	db              database.DBTX
	factory         appinterfaces.RepositoryFactory
	retentionPeriod time.Duration
	o11y            observability.Observability
}

func NewCleanupOnboardingTables(
	db database.DBTX,
	factory appinterfaces.RepositoryFactory,
	retentionPeriod time.Duration,
	o11y observability.Observability,
) *CleanupOnboardingTables {
	return &CleanupOnboardingTables{
		db:              db,
		factory:         factory,
		retentionPeriod: retentionPeriod,
		o11y:            o11y,
	}
}

func (uc *CleanupOnboardingTables) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.cleanup_onboarding_tables")
	defer span.End()

	before := time.Now().UTC().Add(-uc.retentionPeriod)
	repo := uc.factory.OnboardingCleanupRepository(uc.db)

	if err := uc.deleteMetaProcessed(ctx, repo, before); err != nil {
		return err
	}
	return uc.deleteConsumerLookupAttempts(ctx, repo, before)
}

func (uc *CleanupOnboardingTables) deleteMetaProcessed(
	ctx context.Context,
	repo appinterfaces.OnboardingCleanupRepository,
	before time.Time,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("onboarding: cleanup meta_processed_messages: context cancelled: %w", err)
		}
		deleted, err := repo.DeleteMetaProcessedOlderThan(ctx, before, cleanupBatchSize)
		if err != nil {
			return fmt.Errorf("onboarding: cleanup meta_processed_messages: %w", err)
		}
		if deleted < int64(cleanupBatchSize) {
			break
		}
	}
	return nil
}

func (uc *CleanupOnboardingTables) deleteConsumerLookupAttempts(
	ctx context.Context,
	repo appinterfaces.OnboardingCleanupRepository,
	before time.Time,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("onboarding: cleanup consumer_lookup_attempts: context cancelled: %w", err)
		}
		deleted, err := repo.DeleteConsumerLookupAttemptsOlderThan(ctx, before, cleanupBatchSize)
		if err != nil {
			return fmt.Errorf("onboarding: cleanup consumer_lookup_attempts: %w", err)
		}
		if deleted < int64(cleanupBatchSize) {
			break
		}
	}
	return nil
}
