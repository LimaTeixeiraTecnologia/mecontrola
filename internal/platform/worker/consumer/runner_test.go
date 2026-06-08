package consumer_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
	consumermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer/mocks"
)

type RunnerSuite struct {
	suite.Suite
	logger *slog.Logger
}

func TestRunnerSuite(t *testing.T) {
	suite.Run(t, new(RunnerSuite))
}

func (s *RunnerSuite) SetupTest() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *RunnerSuite) TestStart() {
	type args struct {
		ctx context.Context
	}

	type observed struct {
		dispatched []string
	}

	sentinel := errors.New("source error")

	scenarios := []struct {
		name   string
		args   args
		setup  func(*consumermocks.Source, consumer.Registry, *observed)
		expect func(error, observed)
	}{
		{
			name: "deve propagar erro da source",
			args: args{ctx: context.Background()},
			setup: func(source *consumermocks.Source, _ consumer.Registry, _ *observed) {
				source.EXPECT().Start(context.Background(), mock.Anything).Return(sentinel).Once()
			},
			expect: func(err error, _ observed) {
				s.ErrorIs(err, sentinel)
			},
		},
		{
			name: "deve despachar mensagens registradas",
			args: args{ctx: context.Background()},
			setup: func(source *consumermocks.Source, registry consumer.Registry, state *observed) {
				for _, eventType := range []string{"evt1", "evt2"} {
					currentType := eventType
					err := registry.Register(consumer.Registration{
						Name:      currentType,
						EventType: currentType,
						Handler: consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error {
							state.dispatched = append(state.dispatched, currentType)
							return nil
						}),
					})
					s.Require().NoError(err)
				}

				source.EXPECT().Start(context.Background(), mock.Anything).RunAndReturn(
					func(ctx context.Context, deliver func(context.Context, consumer.Message) error) error {
						err := deliver(ctx, consumer.Message{EventType: "evt1"})
						s.Require().NoError(err)
						return deliver(ctx, consumer.Message{EventType: "evt2"})
					},
				).Once()
			},
			expect: func(err error, state observed) {
				s.NoError(err)
				s.Equal([]string{"evt1", "evt2"}, state.dispatched)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := observed{}
			source := consumermocks.NewSource(s.T())
			registry := consumer.NewRegistry()
			scenario.setup(source, registry, &state)

			sut := consumer.NewRunner(source, registry, s.logger)
			err := sut.Start(scenario.args.ctx)

			scenario.expect(err, state)
		})
	}
}

func (s *RunnerSuite) TestStop() {
	type args struct {
		ctx context.Context
	}

	sentinel := errors.New("stop error")

	scenarios := []struct {
		name   string
		args   args
		setup  func(*consumermocks.Source)
		expect func(error)
	}{
		{
			name: "deve chamar stop da source",
			args: args{ctx: context.Background()},
			setup: func(source *consumermocks.Source) {
				source.EXPECT().Stop(context.Background()).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do stop da source",
			args: args{ctx: context.Background()},
			setup: func(source *consumermocks.Source) {
				source.EXPECT().Stop(context.Background()).Return(sentinel).Once()
			},
			expect: func(err error) {
				s.ErrorIs(err, sentinel)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			source := consumermocks.NewSource(s.T())
			scenario.setup(source)

			sut := consumer.NewRunner(source, consumer.NewRegistry(), s.logger)
			err := sut.Stop(scenario.args.ctx)

			scenario.expect(err)
		})
	}
}
