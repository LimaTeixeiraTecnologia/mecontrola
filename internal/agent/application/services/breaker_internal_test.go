package services

import "time"

func SetBreakerClock(b *CircuitBreaker, clock func() time.Time) {
	b.now = clock
}
