package kiwify

import (
	"context"
	"fmt"

	"golang.org/x/time/rate"
)

type rateLimiter struct {
	limiter *rate.Limiter
}

func newRateLimiter(maxRequestsPerMin int, burst int) *rateLimiter {
	rps := rate.Limit(float64(maxRequestsPerMin) / 60.0)
	return &rateLimiter{
		limiter: rate.NewLimiter(rps, burst),
	}
}

func (r *rateLimiter) wait(ctx context.Context) error {
	if err := r.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("billing/kiwify: %w: %w", ErrKiwifyRateLimited, err)
	}
	return nil
}
