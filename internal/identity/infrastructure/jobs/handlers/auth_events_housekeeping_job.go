package handlers

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type cleanupAuthEventsUseCase interface {
	Execute(ctx context.Context) error
}

type AuthEventsHousekeepingJob struct {
	usecase cleanupAuthEventsUseCase
	cfg     configs.IdentityConfig
}

func NewAuthEventsHousekeepingJob(
	uc cleanupAuthEventsUseCase,
	cfg configs.IdentityConfig,
) *AuthEventsHousekeepingJob {
	return &AuthEventsHousekeepingJob{
		usecase: uc,
		cfg:     cfg,
	}
}

func (j *AuthEventsHousekeepingJob) Name() string { return "identity-auth-events-housekeeping" }

func (j *AuthEventsHousekeepingJob) Schedule() string {
	if j.cfg.AuthEventsHousekeepingSchedule != "" {
		return j.cfg.AuthEventsHousekeepingSchedule
	}
	return "@monthly"
}

func (j *AuthEventsHousekeepingJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
