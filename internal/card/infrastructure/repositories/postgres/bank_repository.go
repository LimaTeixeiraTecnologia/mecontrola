package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const fallbackDaysBeforeDue = 7

type bankRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewBankRepository(o11y observability.Observability, db database.DBTX) interfaces.BankDaysReader {
	return &bankRepository{o11y: o11y, db: db}
}

func (r *bankRepository) DaysBeforeDue(ctx context.Context, bank valueobjects.BankCode) (int, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.bank.days_before_due")
	defer span.End()

	const query = `SELECT days_before_due FROM mecontrola.banks WHERE code = $1`

	var days int
	err := r.db.QueryRowContext(ctx, query, bank.LookupKey()).Scan(&days)
	if errors.Is(err, sql.ErrNoRows) {
		return fallbackDaysBeforeDue, nil
	}
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("card.repository.pg: bank.days_before_due: %w", err)
	}
	return days, nil
}

func (r *bankRepository) IsBankRecognized(ctx context.Context, bank valueobjects.BankCode) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.bank.is_recognized")
	defer span.End()

	const query = `SELECT EXISTS(SELECT 1 FROM mecontrola.banks WHERE code = $1)`

	var recognized bool
	err := r.db.QueryRowContext(ctx, query, bank.LookupKey()).Scan(&recognized)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("card.repository.pg: bank.is_recognized: %w", err)
	}
	return recognized, nil
}
