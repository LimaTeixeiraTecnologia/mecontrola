package services_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
)

func newClockedBreaker(t *testing.T, cfg services.CircuitBreakerConfig, now *time.Time) *services.CircuitBreaker {
	t.Helper()
	breaker := services.NewCircuitBreaker(cfg)
	services.SetBreakerClock(breaker, func() time.Time { return *now })
	return breaker
}

func TestCircuitBreaker_StartsClosed(t *testing.T) {
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{})
	state, allowed := breaker.Allow("model-a")
	assert.True(t, allowed)
	assert.Equal(t, services.CircuitClosed, state)
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	clock := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	breaker := newClockedBreaker(t, services.CircuitBreakerConfig{
		MaxFailures:   3,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	}, &clock)

	for i := range 2 {
		state := breaker.RecordFailure("model-a")
		assert.Equal(t, services.CircuitClosed, state, "failure %d", i+1)
	}
	state := breaker.RecordFailure("model-a")
	assert.Equal(t, services.CircuitOpen, state)

	_, allowed := breaker.Allow("model-a")
	assert.False(t, allowed)
}

func TestCircuitBreaker_PrunesOldFailures(t *testing.T) {
	clock := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	breaker := newClockedBreaker(t, services.CircuitBreakerConfig{
		MaxFailures:   3,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	}, &clock)

	breaker.RecordFailure("model-a")
	breaker.RecordFailure("model-a")
	clock = clock.Add(31 * time.Second)
	breaker.RecordFailure("model-a")
	assert.Equal(t, services.CircuitClosed, breaker.State("model-a"))
}

func TestCircuitBreaker_HalfOpenAfterCooldown(t *testing.T) {
	clock := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	breaker := newClockedBreaker(t, services.CircuitBreakerConfig{
		MaxFailures:   2,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	}, &clock)

	breaker.RecordFailure("model-a")
	breaker.RecordFailure("model-a")
	assert.Equal(t, services.CircuitOpen, breaker.State("model-a"))

	_, allowed := breaker.Allow("model-a")
	assert.False(t, allowed)

	clock = clock.Add(61 * time.Second)
	state, allowed := breaker.Allow("model-a")
	assert.True(t, allowed)
	assert.Equal(t, services.CircuitHalfOpen, state)
}

func TestCircuitBreaker_HalfOpenSuccessClosesCircuit(t *testing.T) {
	clock := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	breaker := newClockedBreaker(t, services.CircuitBreakerConfig{
		MaxFailures:   2,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	}, &clock)

	breaker.RecordFailure("model-a")
	breaker.RecordFailure("model-a")
	clock = clock.Add(61 * time.Second)
	_, _ = breaker.Allow("model-a")
	breaker.RecordSuccess("model-a")
	assert.Equal(t, services.CircuitClosed, breaker.State("model-a"))
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	clock := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	breaker := newClockedBreaker(t, services.CircuitBreakerConfig{
		MaxFailures:   2,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	}, &clock)

	breaker.RecordFailure("model-a")
	breaker.RecordFailure("model-a")
	clock = clock.Add(61 * time.Second)
	_, _ = breaker.Allow("model-a")
	state := breaker.RecordFailure("model-a")
	assert.Equal(t, services.CircuitOpen, state)
}

func TestCircuitBreaker_IndependentPerModel(t *testing.T) {
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{MaxFailures: 1})

	breaker.RecordFailure("model-a")
	assert.Equal(t, services.CircuitOpen, breaker.State("model-a"))
	assert.Equal(t, services.CircuitClosed, breaker.State("model-b"))
}

func TestCircuitBreaker_ConcurrentSafe(t *testing.T) {
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{MaxFailures: 100})

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			breaker.RecordFailure("model-a")
		}()
	}
	wg.Wait()
	state := breaker.State("model-a")
	assert.True(t, state == services.CircuitOpen || state == services.CircuitClosed)
}

func TestCircuitState_String(t *testing.T) {
	assert.Equal(t, "closed", services.CircuitClosed.String())
	assert.Equal(t, "open", services.CircuitOpen.String())
	assert.Equal(t, "half_open", services.CircuitHalfOpen.String())
	assert.Equal(t, "invalid", services.CircuitState(0).String())
	assert.Equal(t, "invalid", services.CircuitState(99).String())
}
