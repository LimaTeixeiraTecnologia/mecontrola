package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

var ErrPlanNotFound = errors.New("billing: plan not found")

type planRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewPlanRepository(o11y observability.Observability, db database.DBTX) interfaces.PlanRepository {
	return &planRepository{o11y: o11y, db: db}
}

func (r *planRepository) FindByKiwifyProductID(ctx context.Context, kiwifyProductID string) (valueobjects.Plan, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.plan.find_by_kiwify_product_id")
	defer span.End()

	const query = `
		SELECT code, duration_days
		  FROM billing_plans
		 WHERE kiwify_product_id = $1
	`

	return r.scanPlan(ctx, span, "find_by_kiwify_product_id", r.db.QueryRowContext(ctx, query, kiwifyProductID))
}

func (r *planRepository) FindByCode(ctx context.Context, code valueobjects.PlanCode) (valueobjects.Plan, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.plan.find_by_code")
	defer span.End()

	const query = `
		SELECT code, duration_days
		  FROM billing_plans
		 WHERE code = $1
	`

	return r.scanPlan(ctx, span, "find_by_code", r.db.QueryRowContext(ctx, query, string(code)))
}

func (r *planRepository) ConfigureProductIDs(ctx context.Context, productIDs map[valueobjects.PlanCode]string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.plan.configure_product_ids")
	defer span.End()

	const query = `
		UPDATE billing_plans
		   SET kiwify_product_id = CASE code
		       WHEN 'MONTHLY' THEN $1
		       WHEN 'QUARTERLY' THEN $2
		       WHEN 'ANNUAL' THEN $3
		   END
		 WHERE code IN ('MONTHLY', 'QUARTERLY', 'ANNUAL')
	`
	result, err := r.db.ExecContext(ctx, query,
		productIDs[valueobjects.PlanCodeMonthly],
		productIDs[valueobjects.PlanCodeQuarterly],
		productIDs[valueobjects.PlanCodeAnnual],
	)
	if err != nil {
		return fmt.Errorf("billing/postgres: configure product ids: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("billing/postgres: configure product ids rows_affected: %w", err)
	}
	if rows != 3 {
		return fmt.Errorf("billing/postgres: configure product ids: expected 3 plans, updated %d", rows)
	}
	return nil
}

func (r *planRepository) scanPlan(ctx context.Context, span observability.Span, op string, row *sql.Row) (valueobjects.Plan, error) {
	var code string
	var durationDays int

	err := row.Scan(&code, &durationDays)
	if errors.Is(err, sql.ErrNoRows) {
		return valueobjects.Plan{}, fmt.Errorf("billing/postgres: %s: %w", op, ErrPlanNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "billing.repository.plan.scan_failed",
			observability.String("operation", op),
			observability.Error(err),
		)
		return valueobjects.Plan{}, fmt.Errorf("billing/postgres: %s scan: %w", op, err)
	}

	plan, err := valueobjects.NewPlan(code, durationDays)
	if err != nil {
		span.RecordError(err)
		return valueobjects.Plan{}, fmt.Errorf("billing/postgres: %s plan: %w", op, err)
	}
	return plan, nil
}
