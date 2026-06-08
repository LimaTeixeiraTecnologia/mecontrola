package interfaces

import "context"

type CheckoutURLBuilder interface {
	Build(ctx context.Context, planID, token string) (string, error)
}
