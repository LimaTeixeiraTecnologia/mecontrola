package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type processGraceExpiredUseCase interface {
	Execute(ctx context.Context) error
}

type GraceExpirationJob struct {
	usecase  processGraceExpiredUseCase
	schedule string
}

func NewGraceExpirationJob(uc processGraceExpiredUseCase, cfg configs.BillingConfig) *GraceExpirationJob {
	schedule := cfg.GraceExpirationSchedule
	if schedule == "" {
		schedule = "@every 30m"
	}
	return &GraceExpirationJob{usecase: uc, schedule: schedule}
}

func (j *GraceExpirationJob) Name() string           { return "billing-grace-expiration" }
func (j *GraceExpirationJob) Schedule() string       { return j.schedule }
func (j *GraceExpirationJob) Timeout() time.Duration { return 2 * time.Minute }

func (j *GraceExpirationJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
