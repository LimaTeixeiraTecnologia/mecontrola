package services

import (
	"sync"
	"time"
)

type CircuitState int

const (
	CircuitClosed CircuitState = iota + 1
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "invalid"
	}
}

type CircuitBreakerConfig struct {
	MaxFailures   int
	FailureWindow time.Duration
	OpenDuration  time.Duration
}

type circuitEntry struct {
	mu           sync.Mutex
	state        CircuitState
	failureTimes []time.Time
	openedAt     time.Time
}

type CircuitBreaker struct {
	cfg     CircuitBreakerConfig
	now     func() time.Time
	entries sync.Map
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.FailureWindow <= 0 {
		cfg.FailureWindow = 30 * time.Second
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = 60 * time.Second
	}
	return &CircuitBreaker{cfg: cfg, now: time.Now}
}

func (b *CircuitBreaker) Allow(key string) (CircuitState, bool) {
	entry := b.entryFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := b.now()
	switch entry.state {
	case CircuitOpen:
		if now.Sub(entry.openedAt) >= b.cfg.OpenDuration {
			entry.state = CircuitHalfOpen
			return CircuitHalfOpen, true
		}
		return CircuitOpen, false
	case CircuitHalfOpen:
		return CircuitHalfOpen, true
	default:
		if entry.state == 0 {
			entry.state = CircuitClosed
		}
		return CircuitClosed, true
	}
}

func (b *CircuitBreaker) RecordSuccess(key string) {
	entry := b.entryFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.state = CircuitClosed
	entry.failureTimes = nil
	entry.openedAt = time.Time{}
}

func (b *CircuitBreaker) RecordFailure(key string) CircuitState {
	entry := b.entryFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := b.now()

	if entry.state == CircuitHalfOpen {
		entry.state = CircuitOpen
		entry.openedAt = now
		return CircuitOpen
	}

	entry.failureTimes = pruneWindow(entry.failureTimes, now.Add(-b.cfg.FailureWindow))
	entry.failureTimes = append(entry.failureTimes, now)

	if len(entry.failureTimes) >= b.cfg.MaxFailures {
		entry.state = CircuitOpen
		entry.openedAt = now
		entry.failureTimes = nil
		return CircuitOpen
	}

	if entry.state == 0 {
		entry.state = CircuitClosed
	}
	return entry.state
}

func (b *CircuitBreaker) State(key string) CircuitState {
	entry := b.entryFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.state == 0 {
		return CircuitClosed
	}
	return entry.state
}

func (b *CircuitBreaker) entryFor(key string) *circuitEntry {
	if v, ok := b.entries.Load(key); ok {
		return v.(*circuitEntry)
	}
	fresh := &circuitEntry{state: CircuitClosed}
	actual, _ := b.entries.LoadOrStore(key, fresh)
	return actual.(*circuitEntry)
}

func pruneWindow(times []time.Time, cutoff time.Time) []time.Time {
	keep := times[:0]
	for _, t := range times {
		if !t.Before(cutoff) {
			keep = append(keep, t)
		}
	}
	return keep
}
