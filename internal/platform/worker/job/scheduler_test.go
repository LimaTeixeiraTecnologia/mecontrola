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

func (s *SchedulerSuite) SetupTest() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSchedulerSuite(t *testing.T) {
	suite.Run(t, new(SchedulerSuite))
}

func (s *SchedulerSuite) TestRegister_ScheduleInválido() {
	sched := job.NewScheduler(s.logger)
	a := job.NewAdapter("bad", "nao-e-cron", func(_ context.Context) error { return nil })
	err := sched.Register(a)
	s.Error(err)
}

func (s *SchedulerSuite) TestRegister_ScheduleVálido() {
	sched := job.NewScheduler(s.logger)
	a := job.NewAdapter("ok", "@every 1s", func(_ context.Context) error { return nil })
	err := sched.Register(a)
	s.NoError(err)
}

func (s *SchedulerSuite) TestOverlapSkip_NãoDisparaConcorrente() {
	var running atomic.Int32
	var maxConcurrent atomic.Int32

	sched := job.NewScheduler(s.logger)
	a := job.NewAdapter("slow-job", "@every 1s", func(ctx context.Context) error {
		current := running.Add(1)
		if current > maxConcurrent.Load() {
			maxConcurrent.Store(current)
		}
		time.Sleep(150 * time.Millisecond)
		running.Add(-1)
		return nil
	})
	err := sched.Register(a)
	s.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	time.Sleep(2500 * time.Millisecond)
	cancel()
	<-done
	sched.Stop()

	s.LessOrEqual(maxConcurrent.Load(), int32(1), "OverlapSkip não deve permitir execução concorrente")
}

func (s *SchedulerSuite) TestOverlapAllow_SemGoroutineLeak() {
	var executed atomic.Int32
	executedOnce := make(chan struct{}, 1)
	release := make(chan struct{})
	sched := job.NewScheduler(s.logger)
	a := job.NewAdapterWithPolicy("concurrent-job", "@every 1s", func(ctx context.Context) error {
		executed.Add(1)
		select {
		case executedOnce <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}, job.OverlapAllow)
	err := sched.Register(a)
	s.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	select {
	case <-executedOnce:
	case <-time.After(2 * time.Second):
		s.FailNow("job não executou dentro do prazo")
	}
	cancel()
	<-done
	close(release)
	sched.Stop()

	s.Greater(executed.Load(), int32(0), "job deve ter executado ao menos uma vez")
}

func (s *SchedulerSuite) TestCancelamento_EncerraExecução() {
	ctx, cancel := context.WithCancel(context.Background())
	sched := job.NewScheduler(s.logger)
	a := job.NewAdapter("cancellable", "@every 100ms", func(jobCtx context.Context) error {
		<-jobCtx.Done()
		return nil
	})
	err := sched.Register(a)
	s.NoError(err)

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		s.Fail("scheduler não encerrou após cancelamento do ctx")
	}
}
