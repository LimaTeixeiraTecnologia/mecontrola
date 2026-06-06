package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

type subscriptionProjectionReader struct {
	mgr  manager.Manager
	o11y observability.Observability
}

func NewSubscriptionProjectionReader(
	mgr manager.Manager,
	o11y observability.Observability,
) interfaces.SubscriptionProjectionReader {
	return &subscriptionProjectionReader{mgr: mgr, o11y: o11y}
}

func (r *subscriptionProjectionReader) FindCurrentBySubscriptionID(
	ctx context.Context,
	subscriptionID string,
) (interfaces.SubscriptionProjectionRecord, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.subscription_projection.find_current_by_subscription_id")
	defer span.End()

	const query = `
		SELECT funnel_token, user_id, status, period_end, grace_end, last_event_at
		  FROM billing_subscriptions
		 WHERE id = $1
	`

	var (
		record   interfaces.SubscriptionProjectionRecord
		userID   sql.NullString
		graceEnd sql.NullTime
	)

	err := r.mgr.DBTX(ctx).QueryRowContext(ctx, query, subscriptionID).Scan(
		&record.FunnelToken,
		&userID,
		&record.Status,
		&record.PeriodEnd,
		&graceEnd,
		&record.OccurredAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return interfaces.SubscriptionProjectionRecord{}, fmt.Errorf("identity.repository.subscription_projection.find_current_by_subscription_id: subscription %s not found: %w", subscriptionID, err)
	}
	if err != nil {
		span.RecordError(err)
		return interfaces.SubscriptionProjectionRecord{}, fmt.Errorf("identity.repository.subscription_projection.find_current_by_subscription_id: %w", err)
	}

	record.SubscriptionID = subscriptionID
	if userID.Valid {
		record.UserID = userID.String
	}
	if graceEnd.Valid {
		record.GraceEnd = graceEnd.Time
	}
	return record, nil
}
