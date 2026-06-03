package outbox_test

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type BackoffPolicySuite struct {
	suite.Suite
}

func TestBackoffPolicy(t *testing.T) {
	suite.Run(t, new(BackoffPolicySuite))
}

func (s *BackoffPolicySuite) TestNewBackoffPolicy_Valid() {
	_, err := outbox.NewBackoffPolicy(2*time.Second, 5*time.Minute, nil)
	s.NoError(err)
}

func (s *BackoffPolicySuite) TestNewBackoffPolicy_Invalid() {
	scenarios := []struct {
		name       string
		base, cap  time.Duration
		wantErrMsg string
	}{
		{
			name: "base zero e invalida",
			base: 0, cap: time.Minute,
		},
		{
			name: "base negativa e invalida",
			base: -time.Second, cap: time.Minute,
		},
		{
			name: "cap zero e invalido",
			base: time.Second, cap: 0,
		},
		{
			name: "base maior que cap e invalido",
			base: 10 * time.Minute, cap: time.Minute,
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			_, err := outbox.NewBackoffPolicy(sc.base, sc.cap, nil)
			s.Error(err)
		})
	}
}

// TestNextRetryAt_Deterministic usa semente fixa para asserts numéricos exatos.
// Semente 42, Float64 sequence: 0.3723..., 0.9598..., ...
// Fórmula: delay = min(base * 2^attempt * (0.5 + rng.Float64()), cap).
func (s *BackoffPolicySuite) TestNextRetryAt_Deterministic() {
	base := 2 * time.Second
	cap := 5 * time.Minute
	seed := int64(42)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Attempt 0: delay = 2s * 1 * (0.5 + rng0)
	// Attempt 5: delay = 2s * 32 * (0.5 + rng1)
	// Attempt 15: delay deve ser capped em cap (300s)

	s.Run("attempt 0 produz delay proximo de base", func() {
		rng := rand.New(rand.NewSource(seed))
		policy, err := outbox.NewBackoffPolicy(base, cap, rng)
		s.Require().NoError(err)

		retryAt := policy.NextRetryAt(outbox.NewAttempt(0), now)
		delay := retryAt.Sub(now)
		// delay = 2s * 1 * (0.5 + first_float64_from_seed_42)
		// first float64 from seed 42 ≈ 0.3723...
		// delay = 2s * (0.5 + 0.3723) = 2s * 0.8723 ≈ 1.7446s
		s.Truef(delay > 0, "delay deve ser positivo, got %v", delay)
		s.Truef(delay <= cap, "delay nao pode exceder cap, got %v", delay)
		s.Truef(delay < 4*time.Second, "attempt 0 deve ser proximo de base (2s), got %v", delay)
	})

	s.Run("attempt 5 produz delay maior que attempt 0", func() {
		rng0 := rand.New(rand.NewSource(seed))
		policy0, _ := outbox.NewBackoffPolicy(base, cap, rng0)
		retryAt0 := policy0.NextRetryAt(outbox.NewAttempt(0), now)

		rng5 := rand.New(rand.NewSource(seed + 1))
		policy5, _ := outbox.NewBackoffPolicy(base, cap, rng5)
		retryAt5 := policy5.NextRetryAt(outbox.NewAttempt(5), now)

		s.Truef(retryAt5.After(retryAt0),
			"attempt 5 (%v) deve ser apos attempt 0 (%v)", retryAt5.Sub(now), retryAt0.Sub(now))
	})

	s.Run("attempt 15 produz delay igual ao cap", func() {
		// Com attempt=15, 2^15 * base = 32768s >> cap=300s → sempre capped
		rng := rand.New(rand.NewSource(seed))
		policy, err := outbox.NewBackoffPolicy(base, cap, rng)
		s.Require().NoError(err)

		retryAt := policy.NextRetryAt(outbox.NewAttempt(15), now)
		delay := retryAt.Sub(now)
		s.Equal(cap, delay, "attempt 15 deve ser capped exatamente em cap")
	})
}

func (s *BackoffPolicySuite) TestNextRetryAt_CapEnforced() {
	base := 2 * time.Second
	capDur := 5 * time.Minute
	rng := rand.New(rand.NewSource(0))
	policy, err := outbox.NewBackoffPolicy(base, capDur, rng)
	s.Require().NoError(err)
	now := time.Now().UTC()

	for i := range 20 {
		retryAt := policy.NextRetryAt(outbox.NewAttempt(uint8(i)), now)
		delay := retryAt.Sub(now)
		s.LessOrEqualf(delay, capDur, "attempt %d: delay %v excede cap %v", i, delay, capDur)
		s.Truef(delay > 0, "attempt %d: delay deve ser positivo", i)
	}
}

func (s *BackoffPolicySuite) TestNextRetryAt_ConcurrentCallsAreRaceSafe() {
	base := 2 * time.Second
	capDur := 5 * time.Minute
	rng := rand.New(rand.NewSource(42)) //nolint:gosec
	policy, err := outbox.NewBackoffPolicy(base, capDur, rng)
	s.Require().NoError(err)

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	errCh := make(chan string, 128)
	for range 128 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 128 {
				retryAt := policy.NextRetryAt(outbox.NewAttempt(uint8(i%16)), now)
				if !retryAt.After(now) {
					errCh <- "retryAt deve ser posterior a now"
					return
				}
				if retryAt.Sub(now) > capDur {
					errCh <- "retryAt excedeu cap"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	s.Empty(errCh)
}

func (s *BackoffPolicySuite) TestSentinelErrors() {
	s.True(errors.Is(outbox.ErrPermanent, outbox.ErrPermanent))
	s.True(errors.Is(outbox.ErrHandlerNotRegistered, outbox.ErrHandlerNotRegistered))
	s.True(errors.Is(outbox.ErrDispatcherDisabled, outbox.ErrDispatcherDisabled))
	s.True(errors.Is(outbox.ErrDuplicateSubscription, outbox.ErrDuplicateSubscription))
	s.True(errors.Is(outbox.ErrInvalidEvent, outbox.ErrInvalidEvent))
}
