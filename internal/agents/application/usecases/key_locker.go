package usecases

import "context"

type KeyLocker interface {
	WithKeyLock(ctx context.Context, key string, fn func(context.Context) error) error
}
