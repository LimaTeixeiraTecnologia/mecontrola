package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

type entitlementRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewEntitlementRepository(o11y observability.Observability, db database.DBTX) interfaces.EntitlementRepository {
	return &entitlementRepository{o11y: o11y, db: db}
}

func (r *entitlementRepository) Upsert(ctx context.Context, record interfaces.EntitlementRecord) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.entitlement.upsert")
	defer span.End()

	const query = `
		INSERT INTO identity_entitlements (user_id, subscription_id, status, period_end, grace_end, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			subscription_id = EXCLUDED.subscription_id,
			status          = EXCLUDED.status,
			period_end      = EXCLUDED.period_end,
			grace_end       = EXCLUDED.grace_end,
			updated_at      = EXCLUDED.updated_at
	`

	_, err := r.db.ExecContext(ctx, query,
		record.UserID,
		record.SubscriptionID,
		record.Status,
		record.PeriodEnd,
		sqlnull.Time(record.GraceEnd),
		time.Now().UTC(),
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.entitlement.upsert.failed",
			observability.String("user_id", record.UserID),
			observability.Error(err),
		)
		return fmt.Errorf("identity.repository.entitlement.upsert: %w", err)
	}
	return nil
}

func (r *entitlementRepository) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.entitlement.find_by_user_id")
	defer span.End()

	const query = `
		SELECT user_id, subscription_id, status, period_end, grace_end
		  FROM identity_entitlements
		 WHERE user_id = $1
	`

	var (
		uid, subscriptionID, status string
		periodEnd                   time.Time
		graceEnd                    sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, query, userID).
		Scan(&uid, &subscriptionID, &status, &periodEnd, &graceEnd)
	if errors.Is(err, sql.ErrNoRows) {
		return interfaces.EntitlementRecord{}, application.ErrEntitlementNotFound
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.entitlement.find_by_user_id.failed",
			observability.String("user_id", userID),
			observability.Error(err),
		)
		return interfaces.EntitlementRecord{}, fmt.Errorf("identity.repository.entitlement.find_by_user_id: %w", err)
	}

	record := interfaces.EntitlementRecord{
		UserID:         uid,
		SubscriptionID: subscriptionID,
		Status:         status,
		PeriodEnd:      periodEnd,
	}
	if graceEnd.Valid {
		record.GraceEnd = graceEnd.Time
	}
	return record, nil
}

func (r *entitlementRepository) UpsertPending(ctx context.Context, subscriptionID string, funnelToken string, payload []byte) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.entitlement.upsert_pending")
	defer span.End()

	const query = `
		INSERT INTO identity_entitlements_pending (subscription_id, funnel_token, payload, received_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subscription_id) DO UPDATE SET
			funnel_token = CASE
				WHEN EXCLUDED.funnel_token <> '' THEN EXCLUDED.funnel_token
				ELSE identity_entitlements_pending.funnel_token
			END,
			payload      = EXCLUDED.payload,
			received_at  = EXCLUDED.received_at
	`

	_, err := r.db.ExecContext(ctx, query,
		subscriptionID,
		funnelToken,
		payload,
		time.Now().UTC(),
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.entitlement.upsert_pending.failed",
			observability.String("subscription_id", subscriptionID),
			observability.Error(err),
		)
		return fmt.Errorf("identity.repository.entitlement.upsert_pending: %w", err)
	}
	return nil
}
