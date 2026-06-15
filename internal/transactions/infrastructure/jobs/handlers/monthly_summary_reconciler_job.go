package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type reconcileMonthlySummaryUseCase interface {
	Execute(ctx context.Context) error
}

type MonthlySummaryReconcilerJob struct {
	usecase  reconcileMonthlySummaryUseCase
	schedule string
}

func NewMonthlySummaryReconcilerJob(
	uc reconcileMonthlySummaryUseCase,
	cfg configs.TransactionsConfig,
) *MonthlySummaryReconcilerJob {
	schedule := cfg.MonthlySummaryReconcilerCron
	if schedule == "" {
		schedule = "@daily"
	}
	return &MonthlySummaryReconcilerJob{
		usecase:  uc,
		schedule: schedule,
	}
}

func (j *MonthlySummaryReconcilerJob) Name() string           { return "transactions-monthly-summary-reconciler" }
func (j *MonthlySummaryReconcilerJob) Schedule() string       { return j.schedule }
func (j *MonthlySummaryReconcilerJob) Timeout() time.Duration { return 5 * time.Minute }

func (j *MonthlySummaryReconcilerJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
