package events_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	eventsmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events/mocks"
)

type DispatcherSuite struct {
	suite.Suite
}

func TestDispatcherSuite(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {}

func (s *DispatcherSuite) newEvent(eventType string) events.Event {
	event := eventsmocks.NewEvent(s.T())
	event.EXPECT().GetEventType().Return(eventType)
	return event
}

func (s *DispatcherSuite) TestRegister() {
	type args struct {
		eventType string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(events.Dispatcher) events.Handler
		expect func(events.Dispatcher, events.Handler, error)
	}{
		{
			name:  "deve retornar erro quando event type for vazio",
			args:  args{eventType: ""},
			setup: func(events.Dispatcher) events.Handler { return eventsmocks.NewHandler(s.T()) },
			expect: func(_ events.Dispatcher, _ events.Handler, err error) {
				s.ErrorIs(err, events.ErrEventTypeEmpty)
			},
		},
		{
			name:  "deve retornar erro quando handler for nil",
			args:  args{eventType: "user.created"},
			setup: func(events.Dispatcher) events.Handler { return nil },
			expect: func(_ events.Dispatcher, _ events.Handler, err error) {
				s.ErrorIs(err, events.ErrHandlerNil)
			},
		},
		{
			name: "deve registrar handler valido",
			args: args{eventType: "user.created"},
			setup: func(events.Dispatcher) events.Handler {
				return eventsmocks.NewHandler(s.T())
			},
			expect: func(dispatcher events.Dispatcher, handler events.Handler, err error) {
				s.NoError(err)
				s.True(dispatcher.Has("user.created", handler))
			},
		},
		{
			name: "deve retornar erro ao registrar duplicidade",
			args: args{eventType: "user.created"},
			setup: func(dispatcher events.Dispatcher) events.Handler {
				handler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", handler))
				return handler
			},
			expect: func(_ events.Dispatcher, _ events.Handler, err error) {
				s.ErrorIs(err, events.ErrHandlerAlreadyRegistered)
			},
		},
		{
			name: "deve permitir multiplos handlers do mesmo tipo",
			args: args{eventType: "user.created"},
			setup: func(dispatcher events.Dispatcher) events.Handler {
				firstHandler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", firstHandler))
				return eventsmocks.NewHandler(s.T())
			},
			expect: func(dispatcher events.Dispatcher, handler events.Handler, err error) {
				s.NoError(err)
				s.True(dispatcher.Has("user.created", handler))
				s.Len(dispatcher.HandlersOf("user.created"), 2)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := events.NewDispatcher()
			handler := scenario.setup(sut)

			err := sut.Register(scenario.args.eventType, handler)

			scenario.expect(sut, handler, err)
		})
	}
}

func (s *DispatcherSuite) TestDispatch() {
	type args struct {
		ctx    context.Context
		cancel context.CancelFunc
		event  events.Event
	}

	type observed struct {
		order []int
	}

	handlerErr := errors.New("handler error")

	scenarios := []struct {
		name   string
		args   args
		setup  func(args, events.Dispatcher, *observed)
		expect func(error, observed)
	}{
		{
			name:  "deve retornar erro quando evento for nil",
			args:  args{ctx: context.Background()},
			setup: func(args, events.Dispatcher, *observed) {},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, events.ErrEventNil)
			},
		},
		{
			name:  "deve retornar erro quando event type for vazio",
			args:  args{ctx: context.Background(), event: s.newEvent("")},
			setup: func(args, events.Dispatcher, *observed) {},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, events.ErrEventTypeEmpty)
			},
		},
		{
			name:  "deve ignorar dispatch sem handlers",
			args:  args{ctx: context.Background(), event: s.newEvent("user.created")},
			setup: func(args, events.Dispatcher, *observed) {},
			expect: func(err error, _ observed) {
				s.NoError(err)
			},
		},
		{
			name: "deve despachar com sucesso",
			args: args{ctx: context.Background(), event: s.newEvent("user.created")},
			setup: func(_ args, dispatcher events.Dispatcher, _ *observed) {
				handler := eventsmocks.NewHandler(s.T())
				handler.EXPECT().Handle(context.Background(), mock.Anything).Return(nil).Once()
				s.Require().NoError(dispatcher.Register("user.created", handler))
			},
			expect: func(err error, _ observed) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do handler",
			args: args{ctx: context.Background(), event: s.newEvent("user.created")},
			setup: func(_ args, dispatcher events.Dispatcher, _ *observed) {
				handler := eventsmocks.NewHandler(s.T())
				handler.EXPECT().Handle(context.Background(), mock.Anything).Return(handlerErr).Once()
				s.Require().NoError(dispatcher.Register("user.created", handler))
			},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, handlerErr)
			},
		},
		{
			name: "deve respeitar ordem de execucao",
			args: args{ctx: context.Background(), event: s.newEvent("order.placed")},
			setup: func(_ args, dispatcher events.Dispatcher, state *observed) {
				for index := range 3 {
					orderIndex := index + 1
					handler := eventsmocks.NewHandler(s.T())
					handler.EXPECT().Handle(context.Background(), mock.Anything).Run(func(context.Context, events.Event) {
						state.order = append(state.order, orderIndex)
					}).Return(nil).Once()
					s.Require().NoError(dispatcher.Register("order.placed", handler))
				}
			},
			expect: func(err error, state observed) {
				s.NoError(err)
				s.Equal([]int{1, 2, 3}, state.order)
			},
		},
		{
			name: "deve interromper execucao ao encontrar erro",
			args: args{ctx: context.Background(), event: s.newEvent("order.placed")},
			setup: func(_ args, dispatcher events.Dispatcher, _ *observed) {
				firstHandler := eventsmocks.NewHandler(s.T())
				firstHandler.EXPECT().Handle(context.Background(), mock.Anything).Return(handlerErr).Once()
				secondHandler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("order.placed", firstHandler))
				s.Require().NoError(dispatcher.Register("order.placed", secondHandler))
			},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, handlerErr)
			},
		},
		{
			name: "deve retornar erro quando contexto ja estiver cancelado",
			args: func() args {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return args{ctx: ctx, cancel: cancel, event: s.newEvent("user.created")}
			}(),
			setup: func(_ args, dispatcher events.Dispatcher, _ *observed) {
				handler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", handler))
			},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, context.Canceled)
			},
		},
		{
			name: "deve retornar erro quando contexto cancelar durante execucao",
			args: func() args {
				ctx, cancel := context.WithCancel(context.Background())
				return args{ctx: ctx, cancel: cancel, event: s.newEvent("user.created")}
			}(),
			setup: func(input args, dispatcher events.Dispatcher, _ *observed) {
				firstHandler := eventsmocks.NewHandler(s.T())
				firstHandler.EXPECT().Handle(mock.Anything, mock.Anything).Run(func(context.Context, events.Event) {
					input.cancel()
				}).Return(nil).Once()
				secondHandler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", firstHandler))
				s.Require().NoError(dispatcher.Register("user.created", secondHandler))
			},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, context.Canceled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := observed{}
			sut := events.NewDispatcher()
			scenario.setup(scenario.args, sut, &state)

			err := sut.Dispatch(scenario.args.ctx, scenario.args.event)

			scenario.expect(err, state)
		})
	}
}

func (s *DispatcherSuite) TestCollectionOperations() {
	type observed struct {
		snapshot []events.Handler
	}

	scenarios := []struct {
		name   string
		setup  func(events.Dispatcher, *observed)
		act    func(events.Dispatcher, *observed) error
		expect func(events.Dispatcher, observed, error)
	}{
		{
			name: "deve remover handler existente",
			setup: func(dispatcher events.Dispatcher, _ *observed) {
				handler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", handler))
			},
			act: func(dispatcher events.Dispatcher, _ *observed) error {
				handlers := dispatcher.HandlersOf("user.created")
				return dispatcher.Remove("user.created", handlers[0])
			},
			expect: func(dispatcher events.Dispatcher, _ observed, err error) {
				s.NoError(err)
				s.Empty(dispatcher.HandlersOf("user.created"))
			},
		},
		{
			name: "deve ignorar remocao de handler inexistente",
			setup: func(dispatcher events.Dispatcher, _ *observed) {
				handler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", handler))
			},
			act: func(dispatcher events.Dispatcher, _ *observed) error {
				return dispatcher.Remove("user.created", eventsmocks.NewHandler(s.T()))
			},
			expect: func(dispatcher events.Dispatcher, _ observed, err error) {
				s.NoError(err)
				s.Len(dispatcher.HandlersOf("user.created"), 1)
			},
		},
		{
			name: "deve limpar handlers com clear",
			setup: func(dispatcher events.Dispatcher, _ *observed) {
				s.Require().NoError(dispatcher.Register("user.created", eventsmocks.NewHandler(s.T())))
				s.Require().NoError(dispatcher.Register("order.placed", eventsmocks.NewHandler(s.T())))
			},
			act: func(dispatcher events.Dispatcher, _ *observed) error {
				dispatcher.Clear()
				return nil
			},
			expect: func(dispatcher events.Dispatcher, _ observed, err error) {
				s.NoError(err)
				s.Empty(dispatcher.HandlersOf("user.created"))
				s.Empty(dispatcher.HandlersOf("order.placed"))
			},
		},
		{
			name: "deve retornar snapshot isolado dos handlers",
			setup: func(dispatcher events.Dispatcher, state *observed) {
				handler := eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", handler))
				state.snapshot = dispatcher.HandlersOf("user.created")
			},
			act: func(dispatcher events.Dispatcher, _ *observed) error {
				dispatcher.Clear()
				return nil
			},
			expect: func(_ events.Dispatcher, state observed, err error) {
				s.NoError(err)
				s.Len(state.snapshot, 1)
			},
		},
		{
			name:  "deve criar dispatcher com capacidade customizada",
			setup: func(events.Dispatcher, *observed) {},
			act:   func(_ events.Dispatcher, _ *observed) error { return nil },
			expect: func(_ events.Dispatcher, _ observed, err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := observed{}
			sut := events.NewDispatcher()
			if scenario.name == "deve criar dispatcher com capacidade customizada" {
				sut = events.NewDispatcher(events.WithCapacity(16))
			}
			scenario.setup(sut, &state)

			err := scenario.act(sut, &state)

			scenario.expect(sut, state, err)
		})
	}
}

func (s *DispatcherSuite) TestHas() {
	type args struct {
		eventType string
		handler   events.Handler
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(events.Dispatcher, *args)
		expect func(bool)
	}{
		{
			name: "deve retornar true quando handler existir",
			args: args{eventType: "user.created"},
			setup: func(dispatcher events.Dispatcher, input *args) {
				input.handler = eventsmocks.NewHandler(s.T())
				s.Require().NoError(dispatcher.Register("user.created", input.handler))
			},
			expect: func(result bool) { s.True(result) },
		},
		{
			name:   "deve retornar false quando handler nao existir",
			args:   args{eventType: "user.created", handler: eventsmocks.NewHandler(s.T())},
			setup:  func(events.Dispatcher, *args) {},
			expect: func(result bool) { s.False(result) },
		},
		{
			name:   "deve retornar false com event type vazio",
			args:   args{handler: eventsmocks.NewHandler(s.T())},
			setup:  func(events.Dispatcher, *args) {},
			expect: func(result bool) { s.False(result) },
		},
		{
			name:   "deve retornar false com handler nil",
			args:   args{eventType: "user.created"},
			setup:  func(events.Dispatcher, *args) {},
			expect: func(result bool) { s.False(result) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := events.NewDispatcher()
			scenario.setup(sut, &scenario.args)

			result := sut.Has(scenario.args.eventType, scenario.args.handler)

			scenario.expect(result)
		})
	}
}

func (s *DispatcherSuite) TestConcurrency() {
	scenarios := []struct {
		name   string
		setup  func() []string
		expect func(events.Dispatcher, []string)
	}{
		{
			name: "deve operar de forma concorrente sem panic",
			setup: func() []string {
				eventTypes := make([]string, 0, 20)
				for index := range 20 {
					eventTypes = append(eventTypes, fmt.Sprintf("event.%d", index%5))
				}
				return eventTypes
			},
			expect: func(dispatcher events.Dispatcher, eventTypes []string) {
				var wg sync.WaitGroup
				wg.Add(len(eventTypes) * 4)

				for _, eventType := range eventTypes {
					handler := eventsmocks.NewHandler(s.T())
					handler.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Maybe()
					event := eventsmocks.NewEvent(s.T())
					event.EXPECT().GetEventType().Return(eventType).Maybe()

					go func(eventType string, handler *eventsmocks.Handler, event *eventsmocks.Event) {
						defer wg.Done()
						_ = dispatcher.Register(eventType, handler)
					}(eventType, handler, event)

					go func(event *eventsmocks.Event) {
						defer wg.Done()
						_ = dispatcher.Dispatch(context.Background(), event)
					}(event)

					go func(eventType string, handler *eventsmocks.Handler) {
						defer wg.Done()
						_ = dispatcher.Has(eventType, handler)
					}(eventType, handler)

					go func(eventType string, handler *eventsmocks.Handler) {
						defer wg.Done()
						_ = dispatcher.Remove(eventType, handler)
					}(eventType, handler)
				}

				wg.Wait()
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := events.NewDispatcher()
			eventTypes := scenario.setup()
			scenario.expect(sut, eventTypes)
		})
	}
}
