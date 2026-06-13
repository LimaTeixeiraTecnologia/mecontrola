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

const subscriptionSelectColumns = `id, funnel_token, kiwify_order_id,
		       plan_code, status, period_start, period_end, grace_end, last_event_at,
		       user_id`

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

	query := `SELECT ` + subscriptionSelectColumns + ` FROM billing_subscriptions WHERE kiwify_order_id = $1`

	row := r.db.QueryRowContext(ctx, query, orderID)
	return r.scanRow(ctx, span, "find_by_order_id", row)
}

func (r *subscriptionRepository) FindByKiwifySubID(ctx context.Context, kiwifySubID string) (entities.Subscription, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.find_by_kiwify_sub_id")
	defer span.End()

	if kiwifySubID == "" {
		return entities.Subscription{}, fmt.Errorf("billing/postgres: find_by_kiwify_sub_id: %w", ErrSubscriptionNotFound)
	}

	query := `SELECT ` + subscriptionSelectColumns + `
		  FROM billing_subscriptions
		 WHERE kiwify_subscription_id = $1
		 ORDER BY last_event_at DESC
		 LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, kiwifySubID)
	return r.scanRow(ctx, span, "find_by_kiwify_sub_id", row)
}

func (r *subscriptionRepository) FindByUserID(ctx context.Context, userID string) (entities.Subscription, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.find_by_user_id")
	defer span.End()

	query := `SELECT ` + subscriptionSelectColumns + `
		  FROM billing_subscriptions
		 WHERE user_id = $1
		   AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING')
		 LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, userID)
	return r.scanRow(ctx, span, "find_by_user_id", row)
}

func (r *subscriptionRepository) UpsertByOrder(ctx context.Context, params interfaces.UpsertByOrderParams) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.upsert_by_order")
	defer span.End()

	const query = `
		INSERT INTO billing_subscriptions
		       (id, funnel_token, user_id, kiwify_order_id, kiwify_subscription_id,
		        external_sale_id, customer_mobile_e164, customer_email,
		        plan_code, status, period_start, period_end, grace_end, last_event_at,
		        created_at, updated_at)
		VALUES (gen_random_uuid(), $1, NULL, $2, NULLIF($3, ''),
		        NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
		        $7, $8, $9, $10, $11, $12, now(), now())
		ON CONFLICT (kiwify_order_id) DO UPDATE SET
		    status                 = EXCLUDED.status,
		    period_end             = EXCLUDED.period_end,
		    grace_end              = EXCLUDED.grace_end,
		    last_event_at          = EXCLUDED.last_event_at,
		    kiwify_subscription_id = COALESCE(billing_subscriptions.kiwify_subscription_id, EXCLUDED.kiwify_subscription_id),
		    external_sale_id       = COALESCE(billing_subscriptions.external_sale_id, EXCLUDED.external_sale_id),
		    customer_mobile_e164   = COALESCE(billing_subscriptions.customer_mobile_e164, EXCLUDED.customer_mobile_e164),
		    customer_email         = COALESCE(billing_subscriptions.customer_email, EXCLUDED.customer_email),
		    updated_at             = now()
	`

	sub := params.Subscription
	_, err := r.db.ExecContext(ctx, query,
		sub.FunnelToken().String(),
		params.OrderID,
		params.KiwifySubID,
		params.ExternalSaleID,
		params.CustomerMobileE164,
		params.CustomerEmail,
		string(sub.Plan().Code()),
		sub.Status().String(),
		params.PeriodStart,
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

func (r *subscriptionRepository) ListPastDueGraceExpired(ctx context.Context, now time.Time, limit int) ([]interfaces.ExpiredGraceCandidate, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing.repository.subscription.list_past_due_grace_expired")
	defer span.End()

	if limit <= 0 {
		limit = 100
	}

	const query = `
		SELECT id, grace_end, last_event_at
		  FROM billing_subscriptions
		 WHERE status    = 'PAST_DUE'
		   AND grace_end IS NOT NULL
		   AND grace_end < $1
		 ORDER BY grace_end ASC
		 LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, now, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("billing/postgres: list_past_due_grace_expired: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []interfaces.ExpiredGraceCandidate
	for rows.Next() {
		var (
			id          string
			graceEnd    sql.NullTime
			lastEventAt time.Time
		)
		if scanErr := rows.Scan(&id, &graceEnd, &lastEventAt); scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("billing/postgres: list_past_due_grace_expired scan: %w", scanErr)
		}
		if !graceEnd.Valid {
			continue
		}
		out = append(out, interfaces.ExpiredGraceCandidate{
			SubscriptionID: id,
			GraceEnd:       graceEnd.Time,
			LastEventAt:    lastEventAt,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("billing/postgres: list_past_due_grace_expired rows: %w", rowsErr)
	}
	return out, nil
}

func (r *subscriptionRepository) scanRow(
	ctx context.Context,
	span observability.Span,
	op string,
	row database.Row,
) (entities.Subscription, error) {
	var (
		id, funnelToken, orderID, planCode, status string
		periodStart, periodEnd, lastEventAt        time.Time
		graceEnd                                   sql.NullTime
		userID                                     sql.NullString
	)

	err := row.Scan(
		&id, &funnelToken, &orderID,
		&planCode, &status, &periodStart, &periodEnd, &graceEnd, &lastEventAt,
		&userID,
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

	_ = orderID

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

	var userIDVal string
	if userID.Valid {
		userIDVal = userID.String
	}

	return entities.HydrateWithUser(id, userIDVal, ft, plan, parsedStatus, periodStart, periodEnd, graceEndVal, lastEventAt), nil
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
