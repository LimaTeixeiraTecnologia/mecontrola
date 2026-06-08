package interfaces

import "context"

type SubscriptionBinder interface {
	BindUser(ctx context.Context, subscriptionID string, userID string) error
}
