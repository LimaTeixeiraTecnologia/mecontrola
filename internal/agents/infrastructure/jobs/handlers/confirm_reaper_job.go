package handlers

import (
	"context"
	"time"
)

type staleSuspendedReaper interface {
	Reap(ctx context.Context) (int64, error)
}

type ConfirmReaperJob struct {
	reaper   staleSuspendedReaper
	schedule string
}

func NewConfirmReaperJob(reaper staleSuspendedReaper, schedule string) *ConfirmReaperJob {
	return &ConfirmReaperJob{reaper: reaper, schedule: schedule}
}

func (j *ConfirmReaperJob) Name() string           { return "agents-confirm-reaper" }
func (j *ConfirmReaperJob) Timeout() time.Duration { return 2 * time.Minute }

func (j *ConfirmReaperJob) Schedule() string {
	if j.schedule != "" {
		return j.schedule
	}
	return "*/5 * * * *"
}

func (j *ConfirmReaperJob) Run(ctx context.Context) error {
	_, err := j.reaper.Reap(ctx)
	return err
}
