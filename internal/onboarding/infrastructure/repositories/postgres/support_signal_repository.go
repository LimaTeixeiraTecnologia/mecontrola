package postgres

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type supportSignalRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewSupportSignalRepository(o11y observability.Observability, db database.DBTX) appinterfaces.SupportSignalRepository {
	return &supportSignalRepository{o11y: o11y, db: db}
}

func (r *supportSignalRepository) Insert(ctx context.Context, signal entities.SupportSignal) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.support_signal.insert")
	defer span.End()

	const query = `
		INSERT INTO onboarding.support_signals
		       (id, kind, payload, occurred_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query,
		signal.ID(),
		signal.Kind().String(),
		signal.Payload(),
		signal.OccurredAt(),
	)
	if err != nil {
		return fmt.Errorf("onboarding: support_signal_repository.insert: %w", err)
	}
	return nil
}
