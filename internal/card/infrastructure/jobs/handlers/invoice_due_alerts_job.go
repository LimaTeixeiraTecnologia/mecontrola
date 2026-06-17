package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type evaluateInvoiceDueAlertsUseCase interface {
	Execute(ctx context.Context) error
}

type InvoiceDueAlertsJob struct {
	usecase evaluateInvoiceDueAlertsUseCase
	cfg     configs.CardConfig
}

func NewInvoiceDueAlertsJob(
	usecase evaluateInvoiceDueAlertsUseCase,
	cfg configs.CardConfig,
) *InvoiceDueAlertsJob {
	return &InvoiceDueAlertsJob{usecase: usecase, cfg: cfg}
}

func (j *InvoiceDueAlertsJob) Name() string           { return "card-invoice-due-alerts" }
func (j *InvoiceDueAlertsJob) Timeout() time.Duration { return 5 * time.Minute }

func (j *InvoiceDueAlertsJob) Schedule() string {
	if j.cfg.InvoiceDueAlertsCron != "" {
		return j.cfg.InvoiceDueAlertsCron
	}
	return "@daily"
}

func (j *InvoiceDueAlertsJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
