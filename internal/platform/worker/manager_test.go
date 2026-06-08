package worker_test

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	workermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/mocks"
)

type ManagerSuite struct {
	suite.Suite
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) SetupTest() {}

func (s *ManagerSuite) TestStart() {
	type args struct {
		ctx context.Context
		cfg worker.Config
	}

	type dependencies struct {
		jobs      []worker.Job
		consumers []worker.Consumer
		started   chan struct{}
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() dependencies
		expect func(*worker.Manager, dependencies, error)
	}{
		{
			name: "deve iniciar e parar com sucesso",
			args: args{
				ctx: context.Background(),
				cfg: worker.Config{ShutdownTimeout: 5 * time.Second},
			},
			setup: func() dependencies {
				jobMock := workermocks.NewJob(s.T())
				consumerMock := workermocks.NewConsumer(s.T())
				started := make(chan struct{}, 1)

				jobMock.EXPECT().Name().Return("job1").Times(3)
				jobMock.EXPECT().Schedule().Return("@every 1h").Once()

				consumerMock.EXPECT().Name().Return("consumer1").Twice()
				consumerMock.EXPECT().Start(mock.Anything).RunAndReturn(func(ctx context.Context) error {
					select {
					case started <- struct{}{}:
					default:
					}
					<-ctx.Done()
					return nil
				}).Once()
				consumerMock.EXPECT().Stop(mock.Anything).Return(nil).Once()

				return dependencies{
					jobs:      []worker.Job{jobMock},
					consumers: []worker.Consumer{consumerMock},
					started:   started,
				}
			},
			expect: func(manager *worker.Manager, deps dependencies, err error) {
				s.NoError(err)
				select {
				case <-deps.started:
				case <-time.After(2 * time.Second):
					s.FailNow("consumer nao iniciou dentro do prazo")
				}
				s.NoError(manager.Stop(context.Background()))
			},
		},
		{
			name: "deve retornar erro para nomes duplicados em jobs",
			args: args{
				ctx: context.Background(),
				cfg: worker.Config{ShutdownTimeout: 5 * time.Second},
			},
			setup: func() dependencies {
				firstJob := workermocks.NewJob(s.T())
				secondJob := workermocks.NewJob(s.T())

				firstJob.EXPECT().Name().Return("mesmo").Twice()
				secondJob.EXPECT().Name().Return("mesmo").Twice()

				return dependencies{jobs: []worker.Job{firstJob, secondJob}}
			},
			expect: func(_ *worker.Manager, _ dependencies, err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro para nomes duplicados em consumers",
			args: args{
				ctx: context.Background(),
				cfg: worker.Config{ShutdownTimeout: 5 * time.Second},
			},
			setup: func() dependencies {
				firstConsumer := workermocks.NewConsumer(s.T())
				secondConsumer := workermocks.NewConsumer(s.T())

				firstConsumer.EXPECT().Name().Return("mesmo").Twice()
				secondConsumer.EXPECT().Name().Return("mesmo").Twice()

				return dependencies{consumers: []worker.Consumer{firstConsumer, secondConsumer}}
			},
			expect: func(_ *worker.Manager, _ dependencies, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := scenario.setup()
			sut := worker.NewManager(scenario.args.cfg, deps.jobs, deps.consumers, noopLogger())
			err := sut.Start(scenario.args.ctx)
			scenario.expect(sut, deps, err)
		})
	}
}

func (s *ManagerSuite) TestStop() {
	type args struct {
		ctx context.Context
		cfg worker.Config
	}

	type dependencies struct {
		consumers []worker.Consumer
		started   chan struct{}
		stopped   chan struct{}
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() dependencies
		expect func(*worker.Manager, dependencies, error)
	}{
		{
			name: "deve chamar stop dos consumers",
			args: args{
				ctx: context.Background(),
				cfg: worker.Config{ShutdownTimeout: 5 * time.Second},
			},
			setup: func() dependencies {
				consumerMock := workermocks.NewConsumer(s.T())
				started := make(chan struct{}, 1)
				stopped := make(chan struct{}, 1)

				consumerMock.EXPECT().Name().Return("consumer1").Twice()
				consumerMock.EXPECT().Start(mock.Anything).RunAndReturn(func(ctx context.Context) error {
					select {
					case started <- struct{}{}:
					default:
					}
					<-ctx.Done()
					return nil
				}).Once()
				consumerMock.EXPECT().Stop(mock.Anything).RunAndReturn(func(context.Context) error {
					select {
					case stopped <- struct{}{}:
					default:
					}
					return nil
				}).Once()

				return dependencies{
					consumers: []worker.Consumer{consumerMock},
					started:   started,
					stopped:   stopped,
				}
			},
			expect: func(manager *worker.Manager, deps dependencies, err error) {
				s.NoError(err)
				select {
				case <-deps.started:
				case <-time.After(2 * time.Second):
					s.FailNow("consumer nao iniciou dentro do prazo")
				}

				s.NoError(manager.Stop(context.Background()))

				select {
				case <-deps.stopped:
				case <-time.After(2 * time.Second):
					s.FailNow("consumer stop nao foi chamado")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := scenario.setup()
			sut := worker.NewManager(scenario.args.cfg, nil, deps.consumers, noopLogger())
			err := sut.Start(scenario.args.ctx)
			scenario.expect(sut, deps, err)
		})
	}
}

func (s *ManagerSuite) TestNoGoroutineLeak() {
	type args struct {
		cfg worker.Config
	}

	type dependencies struct {
		consumers []worker.Consumer
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() dependencies
		expect func(int, int, error)
	}{
		{
			name: "deve encerrar sem leak relevante de goroutines",
			args: args{cfg: worker.Config{ShutdownTimeout: 5 * time.Second}},
			setup: func() dependencies {
				consumerMock := workermocks.NewConsumer(s.T())
				consumerMock.EXPECT().Name().Return("consumer1").Twice()
				consumerMock.EXPECT().Start(mock.Anything).RunAndReturn(func(ctx context.Context) error {
					<-ctx.Done()
					return nil
				}).Once()
				consumerMock.EXPECT().Stop(mock.Anything).Return(nil).Once()

				return dependencies{consumers: []worker.Consumer{consumerMock}}
			},
			expect: func(before int, after int, err error) {
				s.NoError(err)
				s.LessOrEqual(after, before+2)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := scenario.setup()
			before := runtime.NumGoroutine()

			sut := worker.NewManager(scenario.args.cfg, nil, deps.consumers, noopLogger())
			err := sut.Start(context.Background())
			s.Require().NoError(err)
			err = sut.Stop(context.Background())

			time.Sleep(100 * time.Millisecond)
			after := runtime.NumGoroutine()

			scenario.expect(before, after, err)
		})
	}
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
