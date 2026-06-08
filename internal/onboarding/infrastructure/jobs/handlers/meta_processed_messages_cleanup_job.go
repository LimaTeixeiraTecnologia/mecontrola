package handlers

import (
	"context"
)

type cleanupOnboardingTablesUseCase interface {
	Execute(ctx context.Context) error
}

type MetaProcessedMessagesCleanupJob struct {
	usecase  cleanupOnboardingTablesUseCase
	schedule string
}

func NewMetaProcessedMessagesCleanupJob(uc cleanupOnboardingTablesUseCase, schedule string) *MetaProcessedMessagesCleanupJob {
	return &MetaProcessedMessagesCleanupJob{usecase: uc, schedule: schedule}
}

func (j *MetaProcessedMessagesCleanupJob) Name() string     { return "onboarding-meta-processed-cleanup" }
func (j *MetaProcessedMessagesCleanupJob) Schedule() string { return j.schedule }

func (j *MetaProcessedMessagesCleanupJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
