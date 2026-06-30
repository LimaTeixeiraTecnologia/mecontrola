package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type subscriptionBoundChecker struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewSubscriptionBoundChecker(o11y observability.Observability, db database.DBTX) appinterfaces.SubscriptionBoundChecker {
	return &subscriptionBoundChecker{o11y: o11y, db: db}
}

func (c *subscriptionBoundChecker) IsAlreadyBound(ctx context.Context, subscriptionID string) (bool, error) {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.repository.subscription_bound_checker.is_already_bound")
	defer span.End()

	const query = `SELECT user_id IS NOT NULL FROM billing_subscriptions WHERE id = $1`

	var bound bool
	err := c.db.QueryRowContext(ctx, query, subscriptionID).Scan(&bound)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("onboarding: subscription_bound_checker.is_already_bound: %w", err)
	}
	return bound, nil
}
