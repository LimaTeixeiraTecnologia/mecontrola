package handlers

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type signalAbandonedDraftsUseCase interface {
	Execute(ctx context.Context) error
}

type AbandonedDraftReaper struct {
	usecase signalAbandonedDraftsUseCase
	cfg     configs.BudgetsConfig
}

func NewAbandonedDraftReaper(
	usecase signalAbandonedDraftsUseCase,
	cfg configs.BudgetsConfig,
) *AbandonedDraftReaper {
	return &AbandonedDraftReaper{usecase: usecase, cfg: cfg}
}

func (j *AbandonedDraftReaper) Name() string { return "budgets-abandoned-draft-reaper" }

func (j *AbandonedDraftReaper) Schedule() string {
	if j.cfg.AbandonedDraftCron != "" {
		return j.cfg.AbandonedDraftCron
	}
	return "0 3 * * *"
}

func (j *AbandonedDraftReaper) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
