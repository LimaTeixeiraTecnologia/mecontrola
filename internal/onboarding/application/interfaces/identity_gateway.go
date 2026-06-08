package interfaces

import "context"

type UpsertUserResult struct {
	UserID string
}

type IdentityGateway interface {
	UpsertUserByWhatsApp(ctx context.Context, mobileE164, email string) (UpsertUserResult, error)
}
