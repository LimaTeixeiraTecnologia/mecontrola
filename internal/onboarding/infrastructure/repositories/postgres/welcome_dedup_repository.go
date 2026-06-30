package postgres

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type welcomeDedupRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewWelcomeDedupRepository(o11y observability.Observability, db database.DBTX) *welcomeDedupRepository {
	return &welcomeDedupRepository{o11y: o11y, db: db}
}

func (r *welcomeDedupRepository) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *welcomeDedupRepository) InsertIfAbsent(ctx context.Context, eventID string) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.welcome_dedup.insert_if_absent")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.onboarding_welcome_processed (event_id, processed_at)
		VALUES ($1, now())
		ON CONFLICT (event_id) DO NOTHING
	`

	result, err := r.conn(ctx).ExecContext(ctx, query, eventID)
	if err != nil {
		return false, fmt.Errorf("onboarding: welcome_dedup_repository.insert_if_absent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("onboarding: welcome_dedup_repository.insert_if_absent rows_affected: %w", err)
	}
	return rows > 0, nil
}
