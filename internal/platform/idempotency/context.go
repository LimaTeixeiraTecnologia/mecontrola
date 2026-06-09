package idempotency

import (
	"context"
	"time"
)

type IdempotencyContext struct {
	Scope       string
	Key         string
	UserID      string
	RequestHash string
	ExpiresAt   time.Time
}

type idempotencyCtxKey struct{}

func WithContext(ctx context.Context, ic IdempotencyContext) context.Context {
	return context.WithValue(ctx, idempotencyCtxKey{}, ic)
}

func FromContext(ctx context.Context) (IdempotencyContext, bool) {
	ic, ok := ctx.Value(idempotencyCtxKey{}).(IdempotencyContext)
	if !ok || ic.Key == "" {
		return IdempotencyContext{}, false
	}
	return ic, true
}
