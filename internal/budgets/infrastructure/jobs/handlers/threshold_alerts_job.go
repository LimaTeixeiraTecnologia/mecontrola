package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type evaluateThresholdAlertsUseCase interface {
	Execute(ctx context.Context) error
}

type ThresholdAlertsJob struct {
	usecase evaluateThresholdAlertsUseCase
	cfg     configs.BudgetsConfig
}

func NewThresholdAlertsJob(
	usecase evaluateThresholdAlertsUseCase,
	cfg configs.BudgetsConfig,
) *ThresholdAlertsJob {
	return &ThresholdAlertsJob{usecase: usecase, cfg: cfg}
}

func (j *ThresholdAlertsJob) Name() string           { return "budgets-threshold-alerts" }
func (j *ThresholdAlertsJob) Timeout() time.Duration { return 5 * time.Minute }

func (j *ThresholdAlertsJob) Schedule() string {
	if j.cfg.ThresholdAlertsCron != "" {
		return j.cfg.ThresholdAlertsCron
	}
	return "@hourly"
}

func (j *ThresholdAlertsJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
