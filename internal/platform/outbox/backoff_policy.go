package outbox

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// BackoffPolicy é o value object que encapsula a política de backoff exponencial
// com jitter. O campo rng é injetável para testes determinísticos (D-13).
//
// BackoffPolicy é seguro para uso concorrente: o RNG injetado é protegido por mutex.
type BackoffPolicy struct {
	base time.Duration
	cap  time.Duration
	rng  *rand.Rand
	mu   *sync.Mutex
}

// NewBackoffPolicy cria uma BackoffPolicy validando as invariantes de domínio.
// Se rng for nil, cria um rand.Rand semeado com time.Now().UnixNano().
func NewBackoffPolicy(base, cap time.Duration, rng *rand.Rand) (BackoffPolicy, error) {
	if base <= 0 {
		return BackoffPolicy{}, fmt.Errorf("outbox: backoff: base deve ser positiva, recebido %v", base)
	}
	if cap <= 0 {
		return BackoffPolicy{}, fmt.Errorf("outbox: backoff: cap deve ser positivo, recebido %v", cap)
	}
	if base > cap {
		return BackoffPolicy{}, fmt.Errorf("outbox: backoff: base (%v) nao pode exceder cap (%v)", base, cap)
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	}
	return BackoffPolicy{base: base, cap: cap, rng: rng, mu: &sync.Mutex{}}, nil
}

// NextRetryAt calcula o instante do próximo retry aplicando backoff exponencial com jitter:
//
//	delay = min(base * 2^attempt * (0.5 + rng.Float64()), cap)
//
// O fator de jitter [0.5, 1.5) evita thundering herd entre múltiplas réplicas.
func (p BackoffPolicy) NextRetryAt(attempt Attempt, now time.Time) time.Time {
	factor := math.Pow(2, float64(attempt.Value()))
	jitter := p.jitter()
	delay := min(time.Duration(float64(p.base)*factor*jitter), p.cap)
	return now.Add(delay)
}

func (p BackoffPolicy) jitter() float64 {
	if p.rng == nil {
		return 1
	}
	if p.mu == nil {
		return 0.5 + p.rng.Float64() // [0.5, 1.5)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return 0.5 + p.rng.Float64() // [0.5, 1.5)
}
