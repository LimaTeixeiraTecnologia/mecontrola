package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/job"
)

type Manager struct {
	cfg       Config
	jobs      []Job
	consumers []Consumer
	logger    *slog.Logger
	scheduler *job.Scheduler
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewManager(cfg Config, jobs []Job, consumers []Consumer, logger *slog.Logger) *Manager {
	if cfg.ShutdownTimeout == 0 {
		cfg = defaultConfig()
	}
	return &Manager{
		cfg:       cfg,
		jobs:      jobs,
		consumers: consumers,
		logger:    logger,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	if err := m.validateNames(); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.scheduler = job.NewScheduler(m.logger)

	for _, j := range m.jobs {
		adapter := job.NewAdapterWithTimeout(j.Name(), j.Schedule(), j.Run, j.Timeout())
		if err := m.scheduler.Register(adapter); err != nil {
			cancel()
			return fmt.Errorf("worker: registrar job %q: %w", j.Name(), err)
		}
	}

	m.wg.Go(func() {
		m.scheduler.Start(runCtx)
	})

	for _, c := range m.consumers {
		m.wg.Add(1)
		c := c
		go func() {
			defer m.wg.Done()
			if err := c.Start(runCtx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				m.logger.ErrorContext(runCtx, "consumer error", "name", c.Name(), "error", err)
			}
		}()
	}

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.scheduler != nil {
		m.scheduler.Stop()
	}

	stopCtx, stopCancel := context.WithTimeout(ctx, m.cfg.ShutdownTimeout)
	defer stopCancel()

	errs := make([]error, 0, len(m.consumers))
	var mu sync.Mutex
	var swg sync.WaitGroup

	for _, c := range m.consumers {
		swg.Add(1)
		c := c
		go func() {
			defer swg.Done()
			if err := c.Stop(stopCtx); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("consumer %q: %w", c.Name(), err))
				mu.Unlock()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		swg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return errors.Join(errs...)
	case <-stopCtx.Done():
		return errors.Join(append(errs, errStopTimeout)...)
	}
}

func (m *Manager) validateNames() error {
	seen := make(map[string]struct{})
	for _, j := range m.jobs {
		if _, ok := seen[j.Name()]; ok {
			return fmt.Errorf("%w: job %q", errDuplicateName, j.Name())
		}
		seen[j.Name()] = struct{}{}
	}
	for _, c := range m.consumers {
		if _, ok := seen[c.Name()]; ok {
			return fmt.Errorf("%w: consumer %q", errDuplicateName, c.Name())
		}
		seen[c.Name()] = struct{}{}
	}
	return nil
}
