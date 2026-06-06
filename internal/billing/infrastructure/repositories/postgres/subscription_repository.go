package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

var ErrConcurrentActiveSub = errors.New("billing: user already has an active subscription")

var ErrSubscriptionNotFound = errors.New("billing: subscription not found")

type subscriptionRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewSubscriptionRepository(o11y observability.Observability, db database.DBTX) interfaces.SubscriptionRepository {
	return &subscriptionRepository{o11y: o11y, db: db}
}

func (r *subscriptionRepository) FindByOrderID(ctx context.Context, orderID string) (entities.Subscription, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.find_by_order_id")
	defer span.End()

	const query = `
		SELECT id, funnel_token, user_id, kiwify_order_id, kiwify_subscription_id,
		       plan_code, status, period_start, period_end, grace_end, last_event_at
		  FROM billing_subscriptions
		 WHERE kiwify_order_id = $1
	`

	row := r.db.QueryRowContext(ctx, query, orderID)
	return r.scanRow(ctx, span, "find_by_order_id", row)
}

func (r *subscriptionRepository) FindByUserID(ctx context.Context, userID string) (entities.Subscription, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.find_by_user_id")
	defer span.End()

	const query = `
		SELECT id, funnel_token, user_id, kiwify_order_id, kiwify_subscription_id,
		       plan_code, status, period_start, period_end, grace_end, last_event_at
		  FROM billing_subscriptions
		 WHERE user_id = $1
		   AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING')
		 LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, userID)
	return r.scanRow(ctx, span, "find_by_user_id", row)
}

func (r *subscriptionRepository) UpsertByOrder(ctx context.Context, orderID string, sub entities.Subscription, periodStart time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.upsert_by_order")
	defer span.End()

	const query = `
		INSERT INTO billing_subscriptions
		       (id, funnel_token, user_id, kiwify_order_id, plan_code, status,
		        period_start, period_end, grace_end, last_event_at, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5,
		        $6, $7, $8, $9, now(), now())
		ON CONFLICT (kiwify_order_id) DO UPDATE SET
		    status        = EXCLUDED.status,
		    period_end    = EXCLUDED.period_end,
		    grace_end     = EXCLUDED.grace_end,
		    last_event_at = EXCLUDED.last_event_at,
		    updated_at    = now()
	`

	_, err := r.db.ExecContext(ctx, query,
		sub.FunnelToken().String(),
		nil,
		orderID,
		string(sub.Plan().Code()),
		sub.Status().String(),
		periodStart,
		sub.PeriodEnd(),
		sqlnull.Time(sub.GraceEnd()),
		sub.LastEventAt(),
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			if pgErr.ConstraintName == "billing_subscriptions_user_active_uniq_idx" {
				return fmt.Errorf("billing/postgres: upsert_by_order: %w", ErrConcurrentActiveSub)
			}
		}
		return fmt.Errorf("billing/postgres: upsert_by_order: %w", err)
	}
	return nil
}

func (r *subscriptionRepository) ExtendPeriod(ctx context.Context, subscriptionID string, newPeriodEnd time.Time, lastEventAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.extend_period")
	defer span.End()

	const query = `
		UPDATE billing_subscriptions
		   SET period_end    = $1,
		       last_event_at = $2,
		       grace_end     = NULL,
		       status        = 'ACTIVE',
		       updated_at    = now()
		 WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query, newPeriodEnd, lastEventAt, subscriptionID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("billing/postgres: extend_period: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("billing/postgres: extend_period rows_affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("billing/postgres: extend_period: %w", ErrSubscriptionNotFound)
	}
	return nil
}

func (r *subscriptionRepository) ApplyTransition(ctx context.Context, subscriptionID string, status valueobjects.Status, graceEnd time.Time, lastEventAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.apply_transition")
	defer span.End()

	const query = `
		UPDATE billing_subscriptions
		   SET status        = $1,
		       grace_end     = $2,
		       last_event_at = $3,
		       updated_at    = now()
		 WHERE id = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		status.String(),
		sqlnull.Time(graceEnd),
		lastEventAt,
		subscriptionID,
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			if pgErr.ConstraintName == "billing_subscriptions_user_active_uniq_idx" {
				return fmt.Errorf("billing/postgres: apply_transition: %w", ErrConcurrentActiveSub)
			}
		}
		return fmt.Errorf("billing/postgres: apply_transition: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("billing/postgres: apply_transition rows_affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("billing/postgres: apply_transition: %w", ErrSubscriptionNotFound)
	}
	return nil
}

func (r *subscriptionRepository) BindUser(ctx context.Context, subscriptionID string, userID string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.bind_user")
	defer span.End()

	const query = `
		UPDATE billing_subscriptions
		   SET user_id    = $1,
		       updated_at = now()
		 WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, userID, subscriptionID)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			if pgErr.ConstraintName == "billing_subscriptions_user_active_uniq_idx" {
				return fmt.Errorf("billing/postgres: bind_user: %w", ErrConcurrentActiveSub)
			}
		}
		return fmt.Errorf("billing/postgres: bind_user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("billing/postgres: bind_user rows_affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("billing/postgres: bind_user: %w", ErrSubscriptionNotFound)
	}
	return nil
}

func (r *subscriptionRepository) scanRow(
	ctx context.Context,
	span observability.Span,
	op string,
	row database.Row,
) (entities.Subscription, error) {
	var (
		id, funnelToken, orderID, planCode, status string
		userID, kiwifySubID                        sql.NullString
		periodStart, periodEnd, lastEventAt        time.Time
		graceEnd                                   sql.NullTime
	)

	_ = orderID
	_ = userID
	_ = kiwifySubID

	err := row.Scan(
		&id, &funnelToken, &userID, &orderID, &kiwifySubID,
		&planCode, &status, &periodStart, &periodEnd, &graceEnd, &lastEventAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Subscription{}, fmt.Errorf("billing/postgres: %s: %w", op, ErrSubscriptionNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "billing.repository.subscription.scan_failed",
			observability.String("operation", op),
			observability.Error(err),
		)
		return entities.Subscription{}, fmt.Errorf("billing/postgres: %s scan: %w", op, err)
	}

	plan, planErr := resolvePlanFromCode(planCode)
	if planErr != nil {
		span.RecordError(planErr)
		return entities.Subscription{}, fmt.Errorf("billing/postgres: %s plan: %w", op, planErr)
	}

	ft, ftErr := valueobjects.NewFunnelToken(funnelToken)
	if ftErr != nil {
		span.RecordError(ftErr)
		return entities.Subscription{}, fmt.Errorf("billing/postgres: %s funnel_token: %w", op, ftErr)
	}

	parsedStatus, statusErr := valueobjects.ParseStatus(status)
	if statusErr != nil {
		span.RecordError(statusErr)
		return entities.Subscription{}, fmt.Errorf("billing/postgres: %s status: %w", op, statusErr)
	}

	var graceEndVal time.Time
	if graceEnd.Valid {
		graceEndVal = graceEnd.Time
	}

	return entities.Hydrate(id, ft, plan, parsedStatus, periodStart, periodEnd, graceEndVal, lastEventAt), nil
}

func resolvePlanFromCode(code string) (valueobjects.Plan, error) {
	switch valueobjects.PlanCode(code) {
	case valueobjects.PlanCodeMonthly:
		return valueobjects.NewPlan(code, 30)
	case valueobjects.PlanCodeQuarterly:
		return valueobjects.NewPlan(code, 90)
	case valueobjects.PlanCodeAnnual:
		return valueobjects.NewPlan(code, 365)
	default:
		return valueobjects.Plan{}, fmt.Errorf("billing/postgres: unknown plan code %q", code)
	}
}
