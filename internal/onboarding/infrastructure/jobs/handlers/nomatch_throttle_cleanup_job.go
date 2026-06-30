package handlers

import (
	"context"
	"time"
)

type cleanupNomatchThrottleUseCase interface {
	Execute(ctx context.Context) error
}

type NomatchThrottleCleanupJob struct {
	usecase  cleanupNomatchThrottleUseCase
	schedule string
}

func NewNomatchThrottleCleanupJob(uc cleanupNomatchThrottleUseCase, schedule string) *NomatchThrottleCleanupJob {
	return &NomatchThrottleCleanupJob{usecase: uc, schedule: schedule}
}

func (j *NomatchThrottleCleanupJob) Name() string           { return "onboarding-nomatch-throttle-cleanup" }
func (j *NomatchThrottleCleanupJob) Timeout() time.Duration { return 2 * time.Minute }

func (j *NomatchThrottleCleanupJob) Schedule() string {
	if j.schedule != "" {
		return j.schedule
	}
	return "@daily"
}

func (j *NomatchThrottleCleanupJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
