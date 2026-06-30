package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type noMatchThrottleRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewNoMatchThrottleRepository(o11y observability.Observability, db database.DBTX) interfaces.NoMatchThrottle {
	return &noMatchThrottleRepository{o11y: o11y, db: db}
}

func (r *noMatchThrottleRepository) AllowReply(ctx context.Context, mobileE164 string, windowStart time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.nomatch_throttle.allow_reply")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.onboarding_activation_nomatch_throttle (mobile_e164, window_start)
		VALUES ($1, $2)
		ON CONFLICT (mobile_e164, window_start) DO NOTHING
	`

	result, err := r.db.ExecContext(ctx, query, mobileE164, windowStart)
	if err != nil {
		return false, fmt.Errorf("onboarding: nomatch_throttle.allow_reply: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("onboarding: nomatch_throttle.allow_reply: rows affected: %w", err)
	}
	return rows > 0, nil
}

func (r *noMatchThrottleRepository) DeleteBefore(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.nomatch_throttle.delete_before")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.onboarding_activation_nomatch_throttle
		 WHERE (mobile_e164, window_start) IN (
			SELECT mobile_e164, window_start
			  FROM mecontrola.onboarding_activation_nomatch_throttle
			 WHERE created_at <= $1
			 ORDER BY created_at
			 LIMIT $2
		)
	`

	result, err := r.db.ExecContext(ctx, query, before, batchSize)
	if err != nil {
		return 0, fmt.Errorf("onboarding: nomatch_throttle.delete_before: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("onboarding: nomatch_throttle.delete_before: rows affected: %w", err)
	}
	return rows, nil
}
