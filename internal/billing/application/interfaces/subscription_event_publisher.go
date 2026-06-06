package interfaces

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
)

type SubscriptionEventPublisher interface {
	PublishActivated(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string, funnelToken string) error
	PublishRenewed(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string, previousPeriodEnd time.Time) error
	PublishPastDue(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string) error
	PublishCanceled(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string) error
	PublishRefunded(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string) error
}
