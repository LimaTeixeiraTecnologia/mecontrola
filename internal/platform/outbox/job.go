package outbox

import (
	"context"
	"math/rand"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type dispatcherJob struct {
	dispatcher *OutboxDispatcher
	schedule   string
}

func NewDispatcherJob(
	storage Storage,
	registry Registry,
	cfg configs.OutboxConfig,
	logger observability.Logger,
	rng *rand.Rand,
) *dispatcherJob {
	return &dispatcherJob{
		dispatcher: NewOutboxDispatcher(storage, registry, cfg, logger, rng),
		schedule:   "@every " + cfg.DispatcherTickInterval.String(),
	}
}

func (j *dispatcherJob) Name() string     { return "outbox-dispatcher" }
func (j *dispatcherJob) Schedule() string { return j.schedule }
func (j *dispatcherJob) Run(ctx context.Context) error {
	return j.dispatcher.RunOnce(ctx)
}

type reaperJob struct {
	runner *reaperRunner
	cfg    configs.OutboxConfig
}

func NewReaperJob(storage Storage, cfg configs.OutboxConfig, logger observability.Logger) *reaperJob {
	return &reaperJob{runner: NewReaperRunner(storage, cfg, logger), cfg: cfg}
}

func (j *reaperJob) Name() string     { return "outbox-reaper" }
func (j *reaperJob) Schedule() string { return j.cfg.ReaperInterval }
func (j *reaperJob) Run(ctx context.Context) error {
	return j.runner.RunOnce(ctx)
}

type housekeepingJob struct {
	runner *housekeepingRunner
	cfg    configs.OutboxConfig
}

func NewHousekeepingJob(storage Storage, cfg configs.OutboxConfig, logger observability.Logger) *housekeepingJob {
	return &housekeepingJob{runner: NewHousekeepingRunner(storage, cfg, logger), cfg: cfg}
}

func (j *housekeepingJob) Name() string     { return "outbox-housekeeping" }
func (j *housekeepingJob) Schedule() string { return j.cfg.HousekeepingSchedule }
func (j *housekeepingJob) Run(ctx context.Context) error {
	return j.runner.RunOnce(ctx)
}
