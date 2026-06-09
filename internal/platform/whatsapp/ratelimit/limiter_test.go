package ratelimit_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

// LimiterSuite cobre os cenários table-driven A–D do techspec.
type LimiterSuite struct {
	suite.Suite
}

func TestLimiterSuite(t *testing.T) {
	suite.Run(t, new(LimiterSuite))
}

func newLimiter() *ratelimit.Limiter {
	return ratelimit.New(noop.NewProvider())
}

// --- Cenários funcionais (A–D) ---

func (s *LimiterSuite) TestAllow_TableDriven() {
	scenarios := []struct {
		name   string
		userID uuid.UUID
		calls  int
		expect []bool // expected result for each call
	}{
		{
			name:   "A - primeiro Allow para user novo retorna true",
			userID: uuid.New(),
			calls:  1,
			expect: []bool{true},
		},
		{
			name:   "B - excede capacidade retorna false",
			userID: uuid.New(),
			calls:  ratelimit.DefaultBucketCapacity + 5,
			expect: func() []bool {
				r := make([]bool, ratelimit.DefaultBucketCapacity+5)
				for i := range ratelimit.DefaultBucketCapacity {
					r[i] = true
				}
				// os últimos 5 devem ser false
				return r
			}(),
		},
		{
			name:   "C - users distintos são isolados",
			userID: uuid.New(),
			calls:  1,
			expect: []bool{true},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			l := newLimiter()
			for i, expected := range sc.expect {
				got := l.Allow(sc.userID)
				s.Equal(expected, got, "call %d", i)
			}
		})
	}
}

func (s *LimiterSuite) TestAllow_DifferentUsers_Isolated() {
	l := newLimiter()
	u1 := uuid.New()
	u2 := uuid.New()

	// esgota u1
	for range ratelimit.DefaultBucketCapacity {
		l.Allow(u1)
	}
	s.False(l.Allow(u1), "u1 deve estar esgotado")
	s.True(l.Allow(u2), "u2 deve ter bucket próprio intacto")
}

func (s *LimiterSuite) TestAllow_RefillAfterWait() {
	l := newLimiter()
	userID := uuid.New()

	// esgota o bucket
	for range ratelimit.DefaultBucketCapacity {
		l.Allow(userID)
	}
	s.False(l.Allow(userID), "deve estar esgotado")

	// aguarda refill (1 token/s → 1.1s garante ao menos 1 token)
	time.Sleep(1100 * time.Millisecond)
	s.True(l.Allow(userID), "após refill deve permitir novamente")
}

// --- Teste de shutdown cooperativo ---

func (s *LimiterSuite) TestShutdown_EncerraDentroDoTimeout() {
	l := newLimiter()
	ctx := context.Background()
	s.NoError(l.Start(ctx))

	before := runtime.NumGoroutine()

	shutdownCtx, cancel := context.WithTimeout(ctx, ratelimit.DefaultShutdownTimeout)
	defer cancel()

	start := time.Now()
	err := l.Shutdown(shutdownCtx)
	elapsed := time.Since(start)

	s.NoError(err)
	s.Less(elapsed, ratelimit.DefaultShutdownTimeout, "Shutdown deve retornar antes do timeout")

	// aguarda o GC reconhecer o encerramento da goroutine
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	s.LessOrEqual(after, before, "goroutine de cleanup deve ter encerrado")
}

func (s *LimiterSuite) TestShutdown_SemStart_RetornaNil() {
	l := newLimiter()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := l.Shutdown(ctx)
	s.NoError(err, "Shutdown sem Start deve retornar nil imediatamente")
	s.Less(time.Since(start), 20*time.Millisecond, "deve retornar quase instantaneamente")
}

func (s *LimiterSuite) TestShutdown_Idempotente() {
	l := newLimiter()
	ctx := context.Background()
	s.NoError(l.Start(ctx))

	shutdownCtx, cancel := context.WithTimeout(ctx, ratelimit.DefaultShutdownTimeout)
	defer cancel()
	s.NoError(l.Shutdown(shutdownCtx))

	ctx2, cancel2 := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel2()
	s.NoError(l.Shutdown(ctx2), "segunda chamada de Shutdown deve ser idempotente")
}

func (s *LimiterSuite) TestStart_Idempotente() {
	l := newLimiter()
	ctx := context.Background()
	s.NoError(l.Start(ctx))
	s.NoError(l.Start(ctx), "segunda chamada de Start deve ser no-op")

	shutdownCtx, cancel := context.WithTimeout(ctx, ratelimit.DefaultShutdownTimeout)
	defer cancel()
	s.NoError(l.Shutdown(shutdownCtx))
}

func TestLimiter_RaceAllow(t *testing.T) {
	t.Parallel()

	l := ratelimit.New(noop.NewProvider())

	const (
		goroutines    = 1000
		callsEach     = 100
		distinctUsers = 20
	)

	users := make([]uuid.UUID, distinctUsers)
	for i := range distinctUsers {
		users[i] = uuid.New()
	}

	var wg sync.WaitGroup
	var allowed atomic.Int64
	var denied atomic.Int64

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range callsEach {
				u := users[j%distinctUsers]
				if l.Allow(u) {
					allowed.Add(1)
				} else {
					denied.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	total := allowed.Load() + denied.Load()
	if total != goroutines*callsEach {
		t.Errorf("total de chamadas inconsistente: got %d, want %d", total, goroutines*callsEach)
	}
	// Cada user tem capacidade de 60 tokens + refill parcial; allowed <= distinctUsers * capacity + refill
	maxAllowed := int64(distinctUsers)*ratelimit.DefaultBucketCapacity + int64(goroutines*callsEach)
	if allowed.Load() > maxAllowed {
		t.Errorf("allowed excede limite teórico: got %d, max %d", allowed.Load(), maxAllowed)
	}
}

// --- Benchmark ---

func BenchmarkLimiter_Allow(b *testing.B) {
	l := ratelimit.New(noop.NewProvider())

	const poolSize = 5000
	users := make([]uuid.UUID, poolSize)
	for i := range poolSize {
		users[i] = uuid.New()
	}

	// pré-aquece os buckets
	for _, u := range users {
		l.Allow(u)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		l.Allow(users[i%poolSize])
	}
}
