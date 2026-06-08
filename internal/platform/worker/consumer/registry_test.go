package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
)

type RegistrySuite struct {
	suite.Suite
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) SetupTest() {}

func (s *RegistrySuite) TestRegister() {
	type args struct {
		registration consumer.Registration
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(consumer.Registry)
		expect func(error)
	}{
		{
			name: "deve registrar handler com sucesso",
			args: args{
				registration: consumer.Registration{
					Name:      "test",
					EventType: "order.created",
					Handler:   consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error { return nil }),
				},
			},
			setup: func(consumer.Registry) {},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando handler for nil",
			args: args{
				registration: consumer.Registration{Name: "test", EventType: "order.created"},
			},
			setup: func(consumer.Registry) {},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro quando event type ja existir",
			args: args{
				registration: consumer.Registration{
					Name:      "test",
					EventType: "order.created",
					Handler:   consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error { return nil }),
				},
			},
			setup: func(registry consumer.Registry) {
				err := registry.Register(consumer.Registration{
					Name:      "test",
					EventType: "order.created",
					Handler:   consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error { return nil }),
				})
				s.Require().NoError(err)
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := consumer.NewRegistry()
			scenario.setup(sut)

			err := sut.Register(scenario.args.registration)

			scenario.expect(err)
		})
	}
}

func (s *RegistrySuite) TestDispatch() {
	type args struct {
		eventType string
		params    map[string]string
		body      []byte
	}

	type observed struct {
		params map[string]string
		body   []byte
	}

	sentinel := errors.New("handler error")

	scenarios := []struct {
		name   string
		args   args
		setup  func(consumer.Registry, *observed)
		expect func(error, observed)
	}{
		{
			name: "deve despachar params e body corretos",
			args: args{
				eventType: "evt",
				params:    map[string]string{"k": "v"},
				body:      []byte("payload"),
			},
			setup: func(registry consumer.Registry, state *observed) {
				err := registry.Register(consumer.Registration{
					Name:      "evt",
					EventType: "evt",
					Handler: consumer.HandlerFunc(func(_ context.Context, params map[string]string, body []byte) error {
						state.params = params
						state.body = body
						return nil
					}),
				})
				s.Require().NoError(err)
			},
			expect: func(err error, state observed) {
				s.NoError(err)
				s.Equal(map[string]string{"k": "v"}, state.params)
				s.Equal([]byte("payload"), state.body)
			},
		},
		{
			name:  "deve retornar erro para event type desconhecido",
			args:  args{eventType: "nao-existe"},
			setup: func(consumer.Registry, *observed) {},
			expect: func(err error, _ observed) {
				s.Error(err)
			},
		},
		{
			name: "deve propagar erro do handler",
			args: args{eventType: "evt"},
			setup: func(registry consumer.Registry, _ *observed) {
				err := registry.Register(consumer.Registration{
					Name:      "evt",
					EventType: "evt",
					Handler: consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error {
						return sentinel
					}),
				})
				s.Require().NoError(err)
			},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, sentinel)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := observed{}
			sut := consumer.NewRegistry()
			scenario.setup(sut, &state)

			err := sut.Dispatch(context.Background(), scenario.args.eventType, scenario.args.params, scenario.args.body)

			scenario.expect(err, state)
		})
	}
}
