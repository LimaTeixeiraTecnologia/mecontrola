package postgres

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type subscriptionBinder struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewSubscriptionBinder(o11y observability.Observability, db database.DBTX) appinterfaces.SubscriptionBinder {
	return &subscriptionBinder{o11y: o11y, db: db}
}

func (b *subscriptionBinder) BindUser(ctx context.Context, subscriptionID string, userID string) error {
	ctx, span := b.o11y.Tracer().Start(ctx, "onboarding.repository.subscription_binder.bind_user")
	defer span.End()

	const query = `
		UPDATE billing_subscriptions
		   SET user_id    = $1,
		       updated_at = now()
		 WHERE id = $2
	`

	result, err := b.db.ExecContext(ctx, query, userID, subscriptionID)
	if err != nil {
		return fmt.Errorf("onboarding: subscription_binder.bind_user: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("onboarding: subscription_binder.bind_user rows_affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("onboarding: subscription_binder.bind_user: subscription not found")
	}
	return nil
}
