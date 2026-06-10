package handlers

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type purgeRetentionUseCase interface {
	Execute(ctx context.Context) error
}

type RetentionPurge struct {
	usecase purgeRetentionUseCase
	cfg     configs.BudgetsConfig
}

func NewRetentionPurge(
	usecase purgeRetentionUseCase,
	cfg configs.BudgetsConfig,
) *RetentionPurge {
	return &RetentionPurge{usecase: usecase, cfg: cfg}
}

func (j *RetentionPurge) Name() string { return "budgets-retention-purge" }

func (j *RetentionPurge) Schedule() string {
	if j.cfg.RetentionPurgeCron != "" {
		return j.cfg.RetentionPurgeCron
	}
	return "0 4 1 * *"
}

func (j *RetentionPurge) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
