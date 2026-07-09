package handlers

import (
	"context"
	"time"
)

type cardCreateStaleSuspendedReaper interface {
	Reap(ctx context.Context) (int64, error)
}

type CardCreateReaperJob struct {
	name     string
	reaper   cardCreateStaleSuspendedReaper
	schedule string
}

func NewCardCreateReaperJob(name string, reaper cardCreateStaleSuspendedReaper, schedule string) *CardCreateReaperJob {
	return &CardCreateReaperJob{name: name, reaper: reaper, schedule: schedule}
}

func (j *CardCreateReaperJob) Name() string {
	if j.name != "" {
		return j.name
	}
	return "agents-card-create-reaper"
}
func (j *CardCreateReaperJob) Timeout() time.Duration { return 2 * time.Minute }

func (j *CardCreateReaperJob) Schedule() string {
	if j.schedule != "" {
		return j.schedule
	}
	return "*/5 * * * *"
}

func (j *CardCreateReaperJob) Run(ctx context.Context) error {
	_, err := j.reaper.Reap(ctx)
	return err
}
