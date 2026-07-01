package handlers

import (
	"context"
	"time"
)

type purgeLedgerUseCase interface {
	Execute(ctx context.Context) error
}

type LedgerRetentionJob struct {
	usecase  purgeLedgerUseCase
	schedule string
}

func NewLedgerRetentionJob(usecase purgeLedgerUseCase, schedule string) *LedgerRetentionJob {
	return &LedgerRetentionJob{usecase: usecase, schedule: schedule}
}

func (j *LedgerRetentionJob) Name() string           { return "agents-ledger-retention" }
func (j *LedgerRetentionJob) Timeout() time.Duration { return 5 * time.Minute }

func (j *LedgerRetentionJob) Schedule() string {
	if j.schedule != "" {
		return j.schedule
	}
	return "0 3 * * *"
}

func (j *LedgerRetentionJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
