package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DispatcherSuite struct {
	suite.Suite
	d Dispatcher
}

func TestDispatcherSuite(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {
	s.d = NewDispatcher()
}

// fakeEvent

type fakeEvent struct {
	eventType string
	payload   any
}

func (e *fakeEvent) GetEventType() string { return e.eventType }
func (e *fakeEvent) GetPayload() any      { return e.payload }

// fakeHandler

type fakeHandler struct {
	calls  []Event
	err    error
	onCall func(ctx context.Context, e Event)
}

func (h *fakeHandler) Handle(ctx context.Context, event Event) error {
	h.calls = append(h.calls, event)
	if h.onCall != nil {
		h.onCall(ctx, event)
	}
	return h.err
}

// Register

func (s *DispatcherSuite) TestRegister_EventTypeVazio() {
	h := &fakeHandler{}
	err := s.d.Register("", h)
	s.ErrorIs(err, ErrEventTypeEmpty)
}

func (s *DispatcherSuite) TestRegister_HandlerNil() {
	err := s.d.Register("user.created", nil)
	s.ErrorIs(err, ErrHandlerNil)
}

func (s *DispatcherSuite) TestRegister_Valido() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))
	s.True(s.d.Has("user.created", h))
}

func (s *DispatcherSuite) TestRegister_Duplicidade() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))
	err := s.d.Register("user.created", h)
	s.ErrorIs(err, ErrHandlerAlreadyRegistered)
}

func (s *DispatcherSuite) TestRegister_MultiploHandlersMesmoTipo() {
	h1 := &fakeHandler{}
	h2 := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h1))
	s.NoError(s.d.Register("user.created", h2))
	s.True(s.d.Has("user.created", h1))
	s.True(s.d.Has("user.created", h2))
}

// Dispatch

func (s *DispatcherSuite) TestDispatch_EventNil() {
	err := s.d.Dispatch(context.Background(), nil)
	s.ErrorIs(err, ErrEventNil)
}

func (s *DispatcherSuite) TestDispatch_EventTypeVazio() {
	err := s.d.Dispatch(context.Background(), &fakeEvent{eventType: ""})
	s.ErrorIs(err, ErrEventTypeEmpty)
}

func (s *DispatcherSuite) TestDispatch_SemHandlers_NoOp() {
	err := s.d.Dispatch(context.Background(), &fakeEvent{eventType: "user.created"})
	s.NoError(err)
}

func (s *DispatcherSuite) TestDispatch_Sucesso() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))
	evt := &fakeEvent{eventType: "user.created", payload: "test"}

	err := s.d.Dispatch(context.Background(), evt)
	s.NoError(err)
	s.Len(h.calls, 1)
	s.Equal(evt, h.calls[0])
}

func (s *DispatcherSuite) TestDispatch_PropagaErroHandler() {
	sentinel := errors.New("handler error")
	h := &fakeHandler{err: sentinel}
	s.NoError(s.d.Register("user.created", h))

	err := s.d.Dispatch(context.Background(), &fakeEvent{eventType: "user.created"})
	s.ErrorIs(err, sentinel)
}

func (s *DispatcherSuite) TestDispatch_OrdemDeExecucao() {
	var order []int
	h1 := &fakeHandler{onCall: func(_ context.Context, _ Event) { order = append(order, 1) }}
	h2 := &fakeHandler{onCall: func(_ context.Context, _ Event) { order = append(order, 2) }}
	h3 := &fakeHandler{onCall: func(_ context.Context, _ Event) { order = append(order, 3) }}

	s.NoError(s.d.Register("order.placed", h1))
	s.NoError(s.d.Register("order.placed", h2))
	s.NoError(s.d.Register("order.placed", h3))

	s.NoError(s.d.Dispatch(context.Background(), &fakeEvent{eventType: "order.placed"}))
	s.Equal([]int{1, 2, 3}, order)
}

func (s *DispatcherSuite) TestDispatch_CurtoCircuitaNoErro() {
	sentinel := errors.New("falha")
	h1 := &fakeHandler{err: sentinel}
	h2 := &fakeHandler{}

	s.NoError(s.d.Register("order.placed", h1))
	s.NoError(s.d.Register("order.placed", h2))

	err := s.d.Dispatch(context.Background(), &fakeEvent{eventType: "order.placed"})
	s.ErrorIs(err, sentinel)
	s.Len(h2.calls, 0)
}

func (s *DispatcherSuite) TestDispatch_ContextCanceladoAntesPrimeiroHandler() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.d.Dispatch(ctx, &fakeEvent{eventType: "user.created"})
	s.ErrorIs(err, context.Canceled)
	s.Len(h.calls, 0)
}

func (s *DispatcherSuite) TestDispatch_ContextCanceladoNoMeio() {
	ctx, cancel := context.WithCancel(context.Background())

	h1 := &fakeHandler{onCall: func(_ context.Context, _ Event) { cancel() }}
	h2 := &fakeHandler{}

	s.NoError(s.d.Register("user.created", h1))
	s.NoError(s.d.Register("user.created", h2))

	err := s.d.Dispatch(ctx, &fakeEvent{eventType: "user.created"})
	s.ErrorIs(err, context.Canceled)
	s.Len(h2.calls, 0)
}

// Remove

func (s *DispatcherSuite) TestRemove_HandlerExistente() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))
	s.NoError(s.d.Remove("user.created", h))
	s.False(s.d.Has("user.created", h))
}

func (s *DispatcherSuite) TestRemove_HandlerInexistente() {
	h1 := &fakeHandler{}
	h2 := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h1))
	s.NoError(s.d.Remove("user.created", h2))
	s.True(s.d.Has("user.created", h1))
}

func (s *DispatcherSuite) TestRemove_EventTypeInexistente() {
	h := &fakeHandler{}
	s.NoError(s.d.Remove("nao.existe", h))
}

func (s *DispatcherSuite) TestRemove_UltimoHandlerLimpaBucket() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))
	s.NoError(s.d.Remove("user.created", h))

	d := s.d.(*dispatcher)
	d.mu.RLock()
	_, ok := d.handlers["user.created"]
	d.mu.RUnlock()
	s.False(ok)
}

func (s *DispatcherSuite) TestRemove_EventTypeVazio_NoOp() {
	h := &fakeHandler{}
	s.NoError(s.d.Remove("", h))
}

func (s *DispatcherSuite) TestRemove_HandlerNil_NoOp() {
	s.NoError(s.d.Remove("user.created", nil))
}

// Has

func (s *DispatcherSuite) TestHas_True() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))
	s.True(s.d.Has("user.created", h))
}

func (s *DispatcherSuite) TestHas_False() {
	h := &fakeHandler{}
	s.False(s.d.Has("user.created", h))
}

func (s *DispatcherSuite) TestHas_EventTypeVazio() {
	h := &fakeHandler{}
	s.False(s.d.Has("", h))
}

func (s *DispatcherSuite) TestHas_HandlerNil() {
	s.False(s.d.Has("user.created", nil))
}

// Clear

func (s *DispatcherSuite) TestClear() {
	h1 := &fakeHandler{}
	h2 := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h1))
	s.NoError(s.d.Register("order.placed", h2))

	s.d.Clear()

	s.False(s.d.Has("user.created", h1))
	s.False(s.d.Has("order.placed", h2))
}

// WithCapacity

func (s *DispatcherSuite) TestWithCapacity_FuncionaCorretamente() {
	d := NewDispatcher(WithCapacity(16))
	h := &fakeHandler{}
	s.NoError(d.Register("user.created", h))
	s.True(d.Has("user.created", h))
}

// safeHandler é um Handler com contador atômico, seguro para uso concorrente em testes.

type safeHandler struct{}

func (h *safeHandler) Handle(_ context.Context, _ Event) error {
	return nil
}

// Concorrência

func (s *DispatcherSuite) TestConcorrencia_Race() {
	d := NewDispatcher()
	const goroutines = 20

	handlers := make([]*safeHandler, goroutines)
	for i := range handlers {
		handlers[i] = &safeHandler{}
	}

	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := range goroutines {
		h := handlers[i]
		eventType := fmt.Sprintf("event.%d", i%5)

		go func() {
			defer wg.Done()
			_ = d.Register(eventType, h)
		}()

		go func() {
			defer wg.Done()
			_ = d.Dispatch(context.Background(), &fakeEvent{eventType: eventType})
		}()

		go func() {
			defer wg.Done()
			_ = d.Has(eventType, h)
		}()

		go func() {
			defer wg.Done()
			_ = d.Remove(eventType, h)
		}()
	}

	wg.Wait()
}

// HandlersOf

func (s *DispatcherSuite) TestHandlersOf_RetornaDoisHandlers() {
	h1 := &fakeHandler{}
	h2 := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h1))
	s.NoError(s.d.Register("user.created", h2))

	handlers := s.d.HandlersOf("user.created")

	s.Len(handlers, 2)
}

func (s *DispatcherSuite) TestHandlersOf_EventTypeVazio_RetornaVazio() {
	handlers := s.d.HandlersOf("")
	s.Empty(handlers)
}

func (s *DispatcherSuite) TestHandlersOf_SnapshotIsolado() {
	h := &fakeHandler{}
	s.NoError(s.d.Register("user.created", h))

	snapshot := s.d.HandlersOf("user.created")
	s.d.Clear()

	s.Len(snapshot, 1, "snapshot deve ser independente do dispatcher")
}
