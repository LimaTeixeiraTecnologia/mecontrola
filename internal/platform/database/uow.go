package database

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	devkituow "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
)

// defaultUoWTimeout is the default timeout applied to every UoW.Do call
// when the caller does not provide a context with an existing deadline.
// Callers can override by passing a context with their own deadline.
const defaultUoWTimeout = 5 * time.Second

// UnitOfWork[T] wraps devkit-go's uow.UnitOfWork[T] adding a mandatory 5-second
// default timeout on every Do call (caller can override via context deadline).
type UnitOfWork[T any] struct {
	inner devkituow.UnitOfWork[T]
}

func NewUnitOfWork[T any](m *Manager) *UnitOfWork[T] {
	return &UnitOfWork[T]{
		inner: devkituow.New[T](m.inner),
	}
}

// Do executes fn inside a transaction.
//
// A 5-second timeout is applied when the ctx has no deadline; callers that
// need a tighter budget should pass a context with their own deadline.
//
// Commit path:  fn returns (value, nil) → Commit → return (value, nil).
// Rollback path: fn returns (_, err) → Rollback → return (zero, err).
// Panic path:   fn panics → recover → Rollback → re-panic.
func (u *UnitOfWork[T]) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (T, error)) (T, error) {
	var zero T

	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultUoWTimeout)
		defer cancel()
	}

	result, err := u.inner.Do(ctx, fn)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return zero, fmt.Errorf("%w: %w", ErrDeadlineExceeded, err)
		}
		return zero, err
	}

	return result, nil
}
