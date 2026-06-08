package postgres

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type metaMessageRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewMetaMessageRepository(o11y observability.Observability, db database.DBTX) appinterfaces.MetaMessageRepository {
	return &metaMessageRepository{o11y: o11y, db: db}
}

func (r *metaMessageRepository) InsertIfAbsent(ctx context.Context, wamid string) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.meta_message.insert_if_absent")
	defer span.End()

	const q = `
		INSERT INTO onboarding.meta_processed_messages (wamid, processed_at)
		VALUES ($1, now())
		ON CONFLICT (wamid) DO NOTHING`

	result, err := r.db.ExecContext(ctx, q, wamid)
	if err != nil {
		return false, fmt.Errorf("onboarding: meta_message: insert_if_absent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("onboarding: meta_message: rows affected: %w", err)
	}
	return rows > 0, nil
}
