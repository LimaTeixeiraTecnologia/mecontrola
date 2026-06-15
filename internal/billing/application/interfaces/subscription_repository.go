package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type UpsertByOrderParams struct {
	OrderID            string
	KiwifySubID        string
	ExternalSaleID     string
	CustomerMobileE164 string
	CustomerEmail      string
	Subscription       entities.Subscription
	PeriodStart        time.Time
}

type ExpiredGraceCandidate struct {
	SubscriptionID string
	UserID         string
	GraceEnd       time.Time
	LastEventAt    time.Time
}

type SubscriptionRepository interface {
	FindByOrderID(ctx context.Context, orderID string) (entities.Subscription, error)
	FindByKiwifySubID(ctx context.Context, kiwifySubID string) (entities.Subscription, error)
	FindByUserID(ctx context.Context, userID string) (entities.Subscription, error)
	UpsertByOrder(ctx context.Context, params UpsertByOrderParams) error
	ExtendPeriod(ctx context.Context, subscriptionID string, newPeriodEnd time.Time, lastEventAt time.Time) error
	ApplyTransition(ctx context.Context, subscriptionID string, status valueobjects.Status, graceEnd time.Time, lastEventAt time.Time) error
	BindUser(ctx context.Context, subscriptionID string, userID string) error
	ListPastDueGraceExpired(ctx context.Context, now time.Time, limit int) ([]ExpiredGraceCandidate, error)
}
