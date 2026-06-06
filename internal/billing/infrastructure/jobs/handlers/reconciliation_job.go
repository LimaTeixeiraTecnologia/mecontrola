package handlers

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type runReconciliationUseCase interface {
	Execute(ctx context.Context) error
}

type ReconciliationJob struct {
	usecase runReconciliationUseCase
	cfg     configs.KiwifyConfig
}

func NewReconciliationJob(
	uc runReconciliationUseCase,
	cfg configs.KiwifyConfig,
) *ReconciliationJob {
	return &ReconciliationJob{
		usecase: uc,
		cfg:     cfg,
	}
}

func (j *ReconciliationJob) Name() string     { return "billing-reconciliation" }
func (j *ReconciliationJob) Schedule() string { return j.cfg.ReconciliationInterval }

func (j *ReconciliationJob) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
