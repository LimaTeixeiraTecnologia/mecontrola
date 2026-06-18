package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type onboardingCleanupRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewOnboardingCleanupRepository(o11y observability.Observability, db database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return &onboardingCleanupRepository{o11y: o11y, db: db}
}

func (r *onboardingCleanupRepository) DeleteMetaProcessedOlderThan(ctx context.Context, before time.Time, limit int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.cleanup.delete_meta_processed")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.channel_processed_messages
		 WHERE (channel, message_id) IN (
		     SELECT channel, message_id FROM mecontrola.channel_processed_messages
		      WHERE channel = 'whatsapp' AND processed_at < $1
		      LIMIT $2
		 )
	`

	result, err := r.db.ExecContext(ctx, query, before, limit)
	if err != nil {
		return 0, fmt.Errorf("onboarding: cleanup_repository.delete_meta_processed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("onboarding: cleanup_repository.delete_meta_processed: rows affected: %w", err)
	}
	return affected, nil
}

func (r *onboardingCleanupRepository) DeleteConsumerLookupAttemptsOlderThan(ctx context.Context, before time.Time, limit int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.cleanup.delete_consumer_lookup_attempts")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.consumer_lookup_attempts
		 WHERE event_id IN (
		     SELECT event_id FROM mecontrola.consumer_lookup_attempts
		      WHERE last_attempt_at < $1
		      LIMIT $2
		 )
	`

	result, err := r.db.ExecContext(ctx, query, before, limit)
	if err != nil {
		return 0, fmt.Errorf("onboarding: cleanup_repository.delete_consumer_lookup_attempts: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("onboarding: cleanup_repository.delete_consumer_lookup_attempts: rows affected: %w", err)
	}
	return affected, nil
}
