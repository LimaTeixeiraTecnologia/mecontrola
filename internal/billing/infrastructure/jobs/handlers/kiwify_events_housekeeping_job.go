package handlers

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type cleanupKiwifyEventsUseCase interface {
	Execute(ctx context.Context) error
}

type KiwifyEventsHousekeepingJob struct {
	usecase cleanupKiwifyEventsUseCase
	cfg     configs.BillingConfig
}

func NewKiwifyEventsHousekeepingJob(
	uc cleanupKiwifyEventsUseCase,
	cfg configs.BillingConfig,
) *KiwifyEventsHousekeepingJob {
	return &KiwifyEventsHousekeepingJob{
		usecase: uc,
		cfg:     cfg,
	}
}

func (j *KiwifyEventsHousekeepingJob) Name() string { return "billing-kiwify-events-housekeeping" }
func (j *KiwifyEventsHousekeepingJob) Schedule() string {
	if j.cfg.KiwifyEventsHousekeepingSchedule != "" {
		return j.cfg.KiwifyEventsHousekeepingSchedule
	}
	return "@daily"
}

func (j *KiwifyEventsHousekeepingJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
