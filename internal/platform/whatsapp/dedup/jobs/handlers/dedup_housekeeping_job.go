package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type cleanupProcessedMessagesUseCase interface {
	Execute(ctx context.Context) error
}

type DedupHousekeepingJob struct {
	usecase cleanupProcessedMessagesUseCase
	cfg     configs.WhatsAppConfig
}

func NewDedupHousekeepingJob(
	uc cleanupProcessedMessagesUseCase,
	cfg configs.WhatsAppConfig,
) *DedupHousekeepingJob {
	return &DedupHousekeepingJob{
		usecase: uc,
		cfg:     cfg,
	}
}

func (j *DedupHousekeepingJob) Name() string           { return "whatsapp-dedup-housekeeping" }
func (j *DedupHousekeepingJob) Timeout() time.Duration { return 2 * time.Minute }

func (j *DedupHousekeepingJob) Schedule() string {
	if j.cfg.DedupHousekeepingSchedule != "" {
		return j.cfg.DedupHousekeepingSchedule
	}
	return "@daily"
}

func (j *DedupHousekeepingJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
