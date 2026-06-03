// Package fakes fornece implementações determinísticas de outbox.Handler
// para uso exclusivo em testes unitários do Dispatcher.
package fakes

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

// SuccessHandler retorna um Handler que sempre retorna nil (sucesso).
// Incrementa callCount atomicamente a cada invocação.
func SuccessHandler(callCount *int64) outbox.Handler {
	return func(_ context.Context, _ outbox.Event) error {
		atomic.AddInt64(callCount, 1)
		return nil
	}
}

// TransientHandler retorna um Handler que sempre retorna um erro transitório (não-permanente).
// Incrementa callCount atomicamente a cada invocação.
func TransientHandler(callCount *int64, msg string) outbox.Handler {
	return func(_ context.Context, _ outbox.Event) error {
		atomic.AddInt64(callCount, 1)
		return errors.New(msg)
	}
}

// PermanentHandler retorna um Handler que retorna ErrPermanent.
// O Dispatcher deve enviar imediatamente para DLQ sem incrementar attempts.
func PermanentHandler(callCount *int64, msg string) outbox.Handler {
	return func(_ context.Context, _ outbox.Event) error {
		atomic.AddInt64(callCount, 1)
		return fmt.Errorf("%s: %w", msg, outbox.ErrPermanent)
	}
}

// PanicHandler retorna um Handler que causa panic.
// O Dispatcher deve capturar via recover e tratar como ErrPermanent → DLQ.
func PanicHandler(callCount *int64, msg string) outbox.Handler {
	return func(_ context.Context, _ outbox.Event) error {
		atomic.AddInt64(callCount, 1)
		panic(msg)
	}
}

// TimeoutHandler retorna um Handler que bloqueia até o ctx ser cancelado,
// simulando um handler que excede o handlerTimeout do Dispatcher.
// Incrementa startCount quando inicia e doneCount quando termina.
func TimeoutHandler(startCount, doneCount *int64) outbox.Handler {
	return func(ctx context.Context, _ outbox.Event) error {
		atomic.AddInt64(startCount, 1)
		defer atomic.AddInt64(doneCount, 1)
		<-ctx.Done()
		return ctx.Err()
	}
}

// BlockingHandler retorna um Handler que bloqueia até que release seja fechado.
// Incrementa startCount quando inicia. Usado para testar Stop com handlers in-flight.
func BlockingHandler(startCount *int64, release <-chan struct{}) outbox.Handler {
	return func(_ context.Context, _ outbox.Event) error {
		atomic.AddInt64(startCount, 1)
		<-release
		return nil
	}
}

// FakeClock é uma implementação de outbox.Clock com tempo controlável.
// Thread-safe via atomic.
type FakeClock struct {
	t atomic.Int64 // nanoseconds since epoch
}

// NewFakeClock cria um FakeClock com o tempo inicial informado.
func NewFakeClock(initial time.Time) *FakeClock {
	c := &FakeClock{}
	c.t.Store(initial.UnixNano())
	return c
}

// Now retorna o instante atual do FakeClock.
func (c *FakeClock) Now() time.Time {
	return time.Unix(0, c.t.Load()).UTC()
}

// Advance avança o FakeClock pelo delta informado.
func (c *FakeClock) Advance(delta time.Duration) {
	c.t.Add(int64(delta))
}

// Set define o instante atual do FakeClock.
func (c *FakeClock) Set(t time.Time) {
	c.t.Store(t.UnixNano())
}
