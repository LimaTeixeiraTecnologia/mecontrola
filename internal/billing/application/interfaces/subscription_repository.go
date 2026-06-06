package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type SubscriptionRepository interface {
	FindByOrderID(ctx context.Context, orderID string) (entities.Subscription, error)
	FindByUserID(ctx context.Context, userID string) (entities.Subscription, error)
	UpsertByOrder(ctx context.Context, orderID string, sub entities.Subscription, periodStart time.Time) error
	ExtendPeriod(ctx context.Context, subscriptionID string, newPeriodEnd time.Time, lastEventAt time.Time) error
	ApplyTransition(ctx context.Context, subscriptionID string, status valueobjects.Status, graceEnd time.Time, lastEventAt time.Time) error
	BindUser(ctx context.Context, subscriptionID string, userID string) error
}
