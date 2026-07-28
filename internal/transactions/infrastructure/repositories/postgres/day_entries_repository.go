package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type dayEntriesRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewDayEntriesRepository(o11y observability.Observability, db database.DBTX) interfaces.DayEntriesRepository {
	return &dayEntriesRepository{o11y: o11y, db: db}
}

func (r *dayEntriesRepository) ListDayEntries(ctx context.Context, userID uuid.UUID, day string) ([]interfaces.MonthlyEntry, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.day_entries.list_day_entries")
	defer span.End()

	if _, err := time.Parse("2006-01-02", day); err != nil {
		return nil, fmt.Errorf("transactions/repository: dia inválido %q: %w", day, err)
	}

	const query = `
		SELECT 'transaction' AS kind, id::text, user_id, ref_month, amount_cents, direction, description,
		       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot, created_at
		  FROM mecontrola.transactions
		 WHERE user_id = $1
		   AND occurred_at >= ($2::date)::timestamp AT TIME ZONE 'UTC'
		   AND occurred_at <  (($2::date + 1))::timestamp AT TIME ZONE 'UTC'
		   AND deleted_at IS NULL
		 ORDER BY created_at ASC, id ASC`

	rows, err := r.db.QueryContext(ctx, query, userID, day)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: listar entradas do dia: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries, scanErr := scanMonthlyEntries(rows)
	if scanErr != nil {
		span.RecordError(scanErr)
		return nil, scanErr
	}
	return entries, nil
}
