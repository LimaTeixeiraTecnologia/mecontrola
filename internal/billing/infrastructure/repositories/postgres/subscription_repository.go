package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

// PgxSubscriptionRepository implementa SubscriptionRepository usando pgx/v5.
// Todas as queries estão parametrizadas em queries.go (sem concatenação de input externo).
type PgxSubscriptionRepository struct {
	mgr    *database.Manager
	mapper rowMapper
}

func NewPgxSubscriptionRepository(mgr *database.Manager) *PgxSubscriptionRepository {
	return &PgxSubscriptionRepository{mgr: mgr}
}

func (r *PgxSubscriptionRepository) dbtx(ctx context.Context) database.DBTX {
	return r.mgr.DBTX(ctx)
}

// FindActiveByUserIDForUpdate executa SELECT ... FOR UPDATE (sem SKIP LOCKED)
// para serializar concurrent processors no mesmo subscription_id (ADR-012).
// Deve ser chamado dentro de uma UnitOfWork ativa.
func (r *PgxSubscriptionRepository) FindActiveByUserIDForUpdate(ctx context.Context, userID identityentities.UserID) (*entities.Subscription, error) {
	row := r.dbtx(ctx).QueryRowContext(ctx, findActiveByUserIDForUpdate, userID.String())
	sub, err := r.scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("postgres subscription repository: find active for update: %w", err)
	}
	return sub, nil
}

func (r *PgxSubscriptionRepository) FindActiveByUserID(ctx context.Context, userID identityentities.UserID) (*entities.Subscription, error) {
	row := r.dbtx(ctx).QueryRowContext(ctx, findActiveByUserID, userID.String())
	sub, err := r.scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("postgres subscription repository: find active: %w", err)
	}
	return sub, nil
}

func (r *PgxSubscriptionRepository) FindByExternalID(ctx context.Context, provider string, externalID valueobjects.ExternalSubscriptionID) (*entities.Subscription, error) {
	row := r.dbtx(ctx).QueryRowContext(ctx, findByExternalID, provider, externalID.String())
	sub, err := r.scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("postgres subscription repository: find by external id: %w", err)
	}
	return sub, nil
}

func (r *PgxSubscriptionRepository) ListByStatusInBatch(
	ctx context.Context,
	statuses []valueobjects.SubscriptionStatus,
	cursorCreatedAt time.Time,
	cursorID entities.SubscriptionID,
	limit int,
) ([]*entities.Subscription, error) {
	placeholders := make([]string, len(statuses))
	args := make([]any, 0, len(statuses)+3)
	for i, s := range statuses {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, s.String())
	}
	n := len(statuses)
	args = append(args, cursorCreatedAt, cursorID.String(), limit)

	query := fmt.Sprintf(`
		SELECT
			id, user_id, provider, external_subscription_id, plan_code, status,
			period_start, period_end, grace_period_end, refund_amount_cents,
			last_event_at, last_webhook_event_id, created_at, updated_at, deleted_at
		FROM subscriptions
		WHERE status IN (%s)
		  AND deleted_at IS NULL
		  AND (created_at, id) > ($%d, $%d)
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, strings.Join(placeholders, ","), n+1, n+2, n+3)

	rows, err := r.dbtx(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres subscription repository: list by status in batch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]*entities.Subscription, 0, limit)
	for rows.Next() {
		sub, scanErr := r.scanSubscription(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres subscription repository: scan row: %w", scanErr)
		}
		result = append(result, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres subscription repository: rows error: %w", err)
	}
	return result, nil
}

func (r *PgxSubscriptionRepository) Upsert(ctx context.Context, sub *entities.Subscription) error {
	var deletedAt *time.Time
	if sub.DeletedAt() != nil {
		t := *sub.DeletedAt()
		deletedAt = &t
	}

	_, err := r.dbtx(ctx).ExecContext(ctx, upsertSubscription,
		sub.ID().String(),
		sub.UserID().String(),
		sub.Provider(),
		sub.ExternalSubscriptionID().String(),
		sub.PlanCode().String(),
		sub.InternalStatus().String(),
		sub.PeriodStart(),
		sub.PeriodEnd(),
		sub.GracePeriodEnd(),
		sub.RefundAmountCents().Cents(),
		sub.LastEventAt(),
		sub.LastWebhookEventID().String(),
		sub.CreatedAt(),
		sub.UpdatedAt(),
		deletedAt,
	)
	if err != nil {
		if r.isUniqueViolation(err) {
			return ErrDuplicateActiveSubscription
		}
		return fmt.Errorf("postgres subscription repository: upsert: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PgxSubscriptionRepository) scanSubscription(row rowScanner) (*entities.Subscription, error) {
	var sr subscriptionRow
	err := row.Scan(
		&sr.ID,
		&sr.UserID,
		&sr.Provider,
		&sr.ExternalSubscriptionID,
		&sr.PlanCode,
		&sr.Status,
		&sr.PeriodStart,
		&sr.PeriodEnd,
		&sr.GracePeriodEnd,
		&sr.RefundAmountCents,
		&sr.LastEventAt,
		&sr.LastWebhookEventID,
		&sr.CreatedAt,
		&sr.UpdatedAt,
		&sr.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return r.mapper.hydrateSubscription(sr)
}

func (r *PgxSubscriptionRepository) isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// LoadKiwifyProductPlans carrega todos os mapeamentos kiwify_product_id → PlanCode
// da tabela billing_plans para popular o BillingPlansRegistry no bootstrap.
func (r *PgxSubscriptionRepository) LoadKiwifyProductPlans(ctx context.Context) (map[string]valueobjects.PlanCode, error) {
	rows, err := r.dbtx(ctx).QueryContext(ctx, loadKiwifyProductPlans)
	if err != nil {
		return nil, fmt.Errorf("postgres subscription repository: carregar planos kiwify: %w", err)
	}
	defer func() { _ = rows.Close() }()

	plans := make(map[string]valueobjects.PlanCode)
	for rows.Next() {
		var productID, planCodeStr string
		if err := rows.Scan(&productID, &planCodeStr); err != nil {
			return nil, fmt.Errorf("postgres subscription repository: scan plano: %w", err)
		}
		planCode, err := valueobjects.ParsePlanCode(planCodeStr)
		if err != nil {
			return nil, fmt.Errorf("postgres subscription repository: plan code inválido %q: %w", planCodeStr, err)
		}
		plans[productID] = planCode
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres subscription repository: iteração planos: %w", err)
	}
	return plans, nil
}
