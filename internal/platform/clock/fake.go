package clock

import (
	"sync"
	"time"
)

// FakeClock é uma implementação controlável de Clock para uso em testes.
// Permite avançar o tempo de forma determinística sem depender de sleep.
type FakeClock struct {
	mu  sync.RWMutex
	now time.Time
}

// NewFakeClock cria um FakeClock fixado no instante fornecido.
// Se t.IsZero(), usa 2024-01-01T00:00:00Z como ponto de partida.
func NewFakeClock(t time.Time) *FakeClock {
	if t.IsZero() {
		t = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return &FakeClock{now: t}
}

// Now retorna o instante atual do FakeClock (controlado pelo teste).
func (f *FakeClock) Now() time.Time {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.now
}

// Advance avança o relógio pelo Duration informado.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// Set define o instante atual do FakeClock para t.
func (f *FakeClock) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}
