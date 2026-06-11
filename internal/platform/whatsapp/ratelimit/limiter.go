package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
)

const (
	DefaultBucketCapacity  = 60
	DefaultRefillPerSecond = 1
	DefaultInactivityTTL   = 5 * time.Minute
	DefaultCleanupPeriod   = 60 * time.Second
	DefaultShutdownTimeout = 5 * time.Second
)

type bucket struct {
	tokens     atomic.Int64
	lastRefill atomic.Int64
	lastSeen   atomic.Int64
}

func newBucket(capacity int) *bucket {
	b := &bucket{}
	b.tokens.Store(int64(capacity))
	now := time.Now().UnixNano()
	b.lastRefill.Store(now)
	b.lastSeen.Store(now)
	return b
}

type Limiter struct {
	buckets       sync.Map
	capacity      int64
	refillPerSec  int64
	inactivityTTL time.Duration
	cleanupPeriod time.Duration
	o11y          observability.Observability
	cleanupHist   observability.Histogram

	started      atomic.Bool
	shutdownCh   chan struct{}
	doneCh       chan struct{}
	startOnce    sync.Once
	shutdownOnce sync.Once
}

func New(o11y observability.Observability) *Limiter {
	l := &Limiter{
		capacity:      DefaultBucketCapacity,
		refillPerSec:  DefaultRefillPerSecond,
		inactivityTTL: DefaultInactivityTTL,
		cleanupPeriod: DefaultCleanupPeriod,
		o11y:          o11y,
		shutdownCh:    make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	_ = o11y.Metrics().Gauge(
		"whatsapp_ratelimit_buckets_count",
		"Numero de buckets ativos no rate limiter de WhatsApp",
		"1",
		func(_ context.Context) float64 {
			var count float64
			l.buckets.Range(func(_, _ any) bool {
				count++
				return true
			})
			return count
		},
	)

	l.cleanupHist = o11y.Metrics().Histogram(
		"whatsapp_ratelimit_cleanup_duration_seconds",
		"Duracao de cada execucao da goroutine de cleanup do rate limiter",
		"s",
	)

	return l
}

func (l *Limiter) Start(_ context.Context) error {
	l.startOnce.Do(func() {
		l.started.Store(true)
		go l.cleanupLoop()
	})
	return nil
}

func (l *Limiter) Shutdown(ctx context.Context) error {
	if !l.started.Load() {
		l.shutdownOnce.Do(func() { close(l.shutdownCh) })
		return nil
	}
	l.shutdownOnce.Do(func() { close(l.shutdownCh) })
	select {
	case <-l.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("whatsapp.ratelimit: shutdown timeout: %w", ctx.Err())
	}
}

func (l *Limiter) Allow(userID uuid.UUID) bool {
	v, ok := l.buckets.Load(userID)
	if !ok {
		nb := newBucket(int(l.capacity))
		v, _ = l.buckets.LoadOrStore(userID, nb)
	}
	b := v.(*bucket) //nolint:forcetypeassert
	now := time.Now().UnixNano()
	b.lastSeen.Store(now)
	l.refill(b, now)
	return l.tryConsume(b)
}

func (l *Limiter) refill(b *bucket, nowNano int64) {
	last := b.lastRefill.Load()
	elapsed := nowNano - last
	if elapsed <= 0 {
		return
	}
	tokensToAdd := elapsed * l.refillPerSec / int64(time.Second)
	if tokensToAdd <= 0 {
		return
	}
	advance := tokensToAdd * int64(time.Second) / l.refillPerSec
	if b.lastRefill.CompareAndSwap(last, last+advance) {
		for {
			current := b.tokens.Load()
			newVal := min(current+tokensToAdd, l.capacity)
			if b.tokens.CompareAndSwap(current, newVal) {
				break
			}
		}
	}
}

func (l *Limiter) tryConsume(b *bucket) bool {
	for {
		current := b.tokens.Load()
		if current <= 0 {
			return false
		}
		if b.tokens.CompareAndSwap(current, current-1) {
			return true
		}
	}
}

func (l *Limiter) cleanupLoop() {
	defer close(l.doneCh)

	ticker := time.NewTicker(l.cleanupPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-l.shutdownCh:
			return
		case <-ticker.C:
			l.runCleanup()
		}
	}
}

func (l *Limiter) runCleanup() {
	ctx := context.Background()
	start := time.Now()

	_, span := l.o11y.Tracer().Start(ctx, "whatsapp.ratelimit.cleanup")
	defer span.End()

	now := time.Now().UnixNano()
	ttlNano := l.inactivityTTL.Nanoseconds()

	var removed, remaining int64
	l.buckets.Range(func(key, value any) bool {
		b := value.(*bucket) //nolint:forcetypeassert
		if now-b.lastSeen.Load() > ttlNano {
			l.buckets.Delete(key)
			removed++
		} else {
			remaining++
		}
		return true
	})

	elapsed := time.Since(start).Seconds()
	l.cleanupHist.Record(ctx, elapsed)

	l.o11y.Logger().Info(
		ctx,
		"whatsapp.ratelimit.cleanup",
		observability.Int64("removed", removed),
		observability.Int64("remaining", remaining),
		observability.Float64("duration_s", elapsed),
	)
}
