package tools

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	maxReadRetryAttempts = 3
	readRetryBaseBackoff = 50 * time.Millisecond
)

func WithReadRetry[T any](ctx context.Context, op func(context.Context) (T, error)) (T, error) {
	var (
		out T
		err error
	)
	for attempt := 1; attempt <= maxReadRetryAttempts; attempt++ {
		out, err = op(ctx)
		if err == nil || !isTransientReadError(err) {
			return out, err
		}
		if attempt == maxReadRetryAttempts {
			break
		}
		backoff := time.Duration(attempt) * readRetryBaseBackoff
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return out, errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}
	return out, err
}

func isTransientReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
