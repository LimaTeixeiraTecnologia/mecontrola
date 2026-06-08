package job_test

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/job"
)

type SchedulerSuite struct {
	suite.Suite
	logger *slog.Logger
}

func TestSchedulerSuite(t *testing.T) {
	suite.Run(t, new(SchedulerSuite))
}

func (s *SchedulerSuite) SetupTest() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *SchedulerSuite) TestRegister() {
	type args struct {
		name     string
		schedule string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(error)
	}{
		{
			name:   "deve retornar erro para schedule invalido",
			args:   args{name: "bad", schedule: "nao-e-cron"},
			setup:  func() {},
			expect: func(err error) { s.Error(err) },
		},
		{
			name:   "deve registrar schedule valido",
			args:   args{name: "ok", schedule: "@every 1s"},
			setup:  func() {},
			expect: func(err error) { s.NoError(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := job.NewScheduler(s.logger)
			adapter := job.NewAdapter(scenario.args.name, scenario.args.schedule, func(context.Context) error { return nil })
			err := sut.Register(adapter)

			scenario.expect(err)
		})
	}
}

func (s *SchedulerSuite) TestStart() {
	type observed struct {
		running       atomic.Int32
		maxConcurrent atomic.Int32
		executed      atomic.Int32
	}

	scenarios := []struct {
		name   string
		setup  func(*observed) (*job.Scheduler, context.CancelFunc, <-chan struct{})
		expect func(*observed, <-chan struct{})
	}{
		{
			name: "deve impedir execucao concorrente com overlap skip",
			setup: func(state *observed) (*job.Scheduler, context.CancelFunc, <-chan struct{}) {
				scheduler := job.NewScheduler(s.logger)
				err := scheduler.Register(job.NewAdapter("slow-job", "@every 1s", func(context.Context) error {
					current := state.running.Add(1)
					if current > state.maxConcurrent.Load() {
						state.maxConcurrent.Store(current)
					}
					time.Sleep(150 * time.Millisecond)
					state.running.Add(-1)
					return nil
				}))
				s.Require().NoError(err)

				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					scheduler.Start(ctx)
					close(done)
				}()

				time.Sleep(2500 * time.Millisecond)
				cancel()

				return scheduler, cancel, done
			},
			expect: func(state *observed, done <-chan struct{}) {
				<-done
				s.LessOrEqual(state.maxConcurrent.Load(), int32(1))
			},
		},
		{
			name: "deve permitir execucao com overlap allow",
			setup: func(state *observed) (*job.Scheduler, context.CancelFunc, <-chan struct{}) {
				executedOnce := make(chan struct{}, 1)
				release := make(chan struct{})
				scheduler := job.NewScheduler(s.logger)
				err := scheduler.Register(job.NewAdapterWithPolicy("concurrent-job", "@every 1s", func(jobCtx context.Context) error {
					state.executed.Add(1)
					select {
					case executedOnce <- struct{}{}:
					default:
					}
					select {
					case <-release:
					case <-jobCtx.Done():
					}
					return nil
				}, job.OverlapAllow))
				s.Require().NoError(err)

				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					scheduler.Start(ctx)
					close(done)
				}()

				select {
				case <-executedOnce:
				case <-time.After(2 * time.Second):
					s.FailNow("job nao executou dentro do prazo")
				}

				cancel()
				<-done
				close(release)

				completed := make(chan struct{})
				close(completed)
				return scheduler, cancel, completed
			},
			expect: func(state *observed, _ <-chan struct{}) {
				s.Greater(state.executed.Load(), int32(0))
			},
		},
		{
			name: "deve encerrar scheduler apos cancelamento",
			setup: func(state *observed) (*job.Scheduler, context.CancelFunc, <-chan struct{}) {
				scheduler := job.NewScheduler(s.logger)
				err := scheduler.Register(job.NewAdapter("cancellable", "@every 100ms", func(jobCtx context.Context) error {
					state.executed.Add(1)
					<-jobCtx.Done()
					return nil
				}))
				s.Require().NoError(err)

				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					scheduler.Start(ctx)
					close(done)
				}()

				time.Sleep(200 * time.Millisecond)
				cancel()

				return scheduler, cancel, done
			},
			expect: func(_ *observed, done <-chan struct{}) {
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					s.FailNow("scheduler nao encerrou apos cancelamento")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := &observed{}
			sut, _, done := scenario.setup(state)
			sut.Stop()
			scenario.expect(state, done)
		})
	}
}
