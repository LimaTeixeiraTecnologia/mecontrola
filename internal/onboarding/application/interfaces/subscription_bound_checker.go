package interfaces

import "context"

type SubscriptionBoundChecker interface {
	IsAlreadyBound(ctx context.Context, subscriptionID string) (bool, error)
}
