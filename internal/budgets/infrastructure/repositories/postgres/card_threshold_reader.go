package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type cardThresholdReader struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewCardThresholdReader(o11y observability.Observability, db database.DBTX) interfaces.CardThresholdReader {
	return &cardThresholdReader{db: db, o11y: o11y}
}

func (r *cardThresholdReader) ListActiveCardsForThresholdScan(ctx context.Context, refMonth valueobjects.Competence, limit int) (out []interfaces.ActiveCardForScan, err error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.card_threshold_reader.list_active")
	defer span.End()

	if limit <= 0 {
		limit = 500
	}

	const query = `
		SELECT c.user_id,
		       c.id          AS card_id,
		       c.limit_cents,
		       COALESCE(i.items_total_cents, 0) AS spent_cents
		  FROM mecontrola.cards c
		  LEFT JOIN mecontrola.transactions_card_invoices i
		    ON i.user_id = c.user_id
		   AND i.card_id = c.id
		   AND i.ref_month = $1
		 WHERE c.deleted_at IS NULL
		   AND c.limit_cents > 0
		 ORDER BY c.user_id, c.id
		 LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, refMonth.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("budgets.repository.card_threshold_reader: query: %w", err)
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	out = make([]interfaces.ActiveCardForScan, 0)
	for rows.Next() {
		var (
			userID, cardID         uuid.UUID
			limitCents, spentCents int64
		)
		if err := rows.Scan(&userID, &cardID, &limitCents, &spentCents); err != nil {
			return nil, fmt.Errorf("budgets.repository.card_threshold_reader: scan: %w", err)
		}
		out = append(out, interfaces.ActiveCardForScan{
			UserID:     userID,
			CardID:     cardID,
			LimitCents: limitCents,
			SpentCents: spentCents,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("budgets.repository.card_threshold_reader: rows err: %w", err)
	}
	return out, nil
}
