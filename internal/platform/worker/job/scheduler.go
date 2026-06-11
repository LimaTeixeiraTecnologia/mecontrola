package job

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/robfig/cron/v3"
)

type runner interface {
	Name() string
	Schedule() string
	Run(context.Context) error
	OverlapPolicy() OverlapPolicy
}

type registeredJob struct {
	name          string
	schedule      string
	overlapPolicy OverlapPolicy
	run           func(context.Context) error
	running       atomic.Bool
}

type Scheduler struct {
	cron    *cron.Cron
	logger  *slog.Logger
	jobs    []registeredJob
	allowWg sync.WaitGroup
}

func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		logger: logger,
	}
}

func (s *Scheduler) Register(j runner) error {
	if _, err := cron.ParseStandard(j.Schedule()); err != nil {
		return fmt.Errorf("schedule inválido para job %q: %w", j.Name(), err)
	}
	s.jobs = append(s.jobs, registeredJob{
		name:          j.Name(),
		schedule:      j.Schedule(),
		overlapPolicy: j.OverlapPolicy(),
		run:           j.Run,
	})
	return nil
}

func (s *Scheduler) Start(ctx context.Context) {
	for i := range s.jobs {
		rj := &s.jobs[i]
		if _, err := s.cron.AddFunc(rj.schedule, func() {
			if rj.overlapPolicy == OverlapSkip {
				if !rj.running.CompareAndSwap(false, true) {
					s.logger.WarnContext(ctx, "job skipped", "name", rj.name, "skipped", true)
					return
				}
				defer rj.running.Store(false)
				if err := rj.run(ctx); err != nil {
					s.logger.ErrorContext(ctx, "job error", "name", rj.name, "error", err)
				}
				return
			}
			s.allowWg.Go(func() {
				if err := rj.run(ctx); err != nil {
					s.logger.ErrorContext(ctx, "job error", "name", rj.name, "error", err)
				}
			})
		}); err != nil {
			s.logger.ErrorContext(ctx, "falha ao registrar job no cron", "name", rj.name, "error", err)
		}
	}
	s.cron.Start()
	<-ctx.Done()
}

func (s *Scheduler) Stop() {
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
	s.allowWg.Wait()
}
